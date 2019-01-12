package pod

import (
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime/pprof"

	"github.com/parallelcointeam/pod/blockchain/indexers"
	"github.com/parallelcointeam/pod/database"
)

const (
	// blockDbNamePrefix is the prefix for the block database name.  The database type is appended to this value to form the full block database name.
	blockDbNamePrefix = "blocks"
)

var (
	cfg *config
)

// winServiceMain is only invoked on Windows.  It detects when pod is running as a service and reacts accordingly.
var winServiceMain func() (bool, error)

// podMain is the real main function for pod.  It is necessary to work around the fact that deferred functions do not run when os.Exit() is called.  The optional serverChan parameter is mainly used by the service code to be notified with the server once it is setup so it can gracefully stop it when requested from the service control manager.
func podMain(serverChan chan<- *server) error {
	// Load configuration and parse command line.  This function also initializes logging and configures it accordingly.
	tcfg, _, err := loadConfig()
	if err != nil {
		return err
	}
	cfg = tcfg
	defer func() {
		if logRotator != nil {
			logRotator.Close()
		}
	}()
	// Get a channel that will be closed when a shutdown signal has been triggered either from an OS signal such as SIGINT (Ctrl+C) or from another subsystem such as the RPC server.
	interrupt := interruptListener()
	defer podLog.Info("Shutdown complete")
	// Show version at startup.
	podLog.Infof("Version %s", version())
	// Enable http profiling server if requested.
	if cfg.Profile != "" {
		go func() {
			listenAddr := net.JoinHostPort("", cfg.Profile)
			podLog.Infof("Profile server listening on %s", listenAddr)
			profileRedirect := http.RedirectHandler("/debug/pprof",
				http.StatusSeeOther)
			http.Handle("/", profileRedirect)
			podLog.Errorf("%v", http.ListenAndServe(listenAddr, nil))
		}()
	}
	// Write cpu profile if requested.
	if cfg.CPUProfile != "" {
		f, err := os.Create(cfg.CPUProfile)
		if err != nil {
			podLog.Errorf("Unable to create cpu profile: %v", err)
			return err
		}
		pprof.StartCPUProfile(f)
		defer f.Close()
		defer pprof.StopCPUProfile()
	}
	// Perform upgrades to pod as new versions require it.
	if err := doUpgrades(); err != nil {
		podLog.Errorf("%v", err)
		return err
	}
	// Return now if an interrupt signal was triggered.
	if interruptRequested(interrupt) {
		return nil
	}
	// Load the block database.
	db, err := loadBlockDB()
	if err != nil {
		podLog.Errorf("%v", err)
		return err
	}
	defer func() {
		// Ensure the database is sync'd and closed on shutdown.
		podLog.Infof("Gracefully shutting down the database...")
		db.Close()
	}()
	// Return now if an interrupt signal was triggered.
	if interruptRequested(interrupt) {
		return nil
	}
	// Drop indexes and exit if requested. NOTE: The order is important here because dropping the tx index also drops the address index since it relies on it.
	if cfg.DropAddrIndex {
		if err := indexers.DropAddrIndex(db, interrupt); err != nil {
			podLog.Errorf("%v", err)
			return err
		}
		return nil
	}
	if cfg.DropTxIndex {
		if err := indexers.DropTxIndex(db, interrupt); err != nil {
			podLog.Errorf("%v", err)
			return err
		}
		return nil
	}
	if cfg.DropCfIndex {
		if err := indexers.DropCfIndex(db, interrupt); err != nil {
			podLog.Errorf("%v", err)
			return err
		}
		return nil
	}
	// Create server and start it.
	server, err := newServer(cfg.Listeners, db, activeNetParams.Params, interrupt, cfg.Algo)
	if err != nil {
		// TODO: this logging could do with some beautifying.
		podLog.Errorf("Unable to start server on %v: %v",
			cfg.Listeners, err)
		return err
	}
	defer func() {
		podLog.Infof("Gracefully shutting down the server...")
		server.Stop()
		server.WaitForShutdown()
		srvrLog.Infof("Server shutdown complete")
	}()
	server.Start()
	if serverChan != nil {
		serverChan <- server
	}
	// Wait until the interrupt signal is received from an OS signal or shutdown is requested through one of the subsystems such as the RPC server.
	<-interrupt
	return nil
}

// removeRegressionDB removes the existing regression test database if running in regression test mode and it already exists.
func removeRegressionDB(dbPath string) error {
	// Don't do anything if not in regression test mode.
	if !cfg.RegressionTest {
		return nil
	}
	// Remove the old regression test database if it already exists.
	fi, err := os.Stat(dbPath)
	if err == nil {
		podLog.Infof("Removing regression test database from '%s'", dbPath)
		if fi.IsDir() {
			err := os.RemoveAll(dbPath)
			if err != nil {
				return err
			}
		} else {
			err := os.Remove(dbPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// dbPath returns the path to the block database given a database type.
func blockDbPath(dbType string) string {
	// The database name is based on the database type.
	dbName := blockDbNamePrefix + "_" + dbType
	if dbType == "sqlite" {
		dbName = dbName + ".db"
	}
	dbPath := filepath.Join(cfg.DataDir, dbName)
	return dbPath
}

// warnMultipleDBs shows a warning if multiple block database types are detected. This is not a situation most users want.  It is handy for development however to support multiple side-by-side databases.
func warnMultipleDBs() {
	// This is intentionally not using the known db types which depend on the database types compiled into the binary since we want to detect legacy db types as well.
	dbTypes := []string{"ffldb", "leveldb", "sqlite"}
	duplicateDbPaths := make([]string, 0, len(dbTypes)-1)
	for _, dbType := range dbTypes {
		if dbType == cfg.DbType {
			continue
		}
		// Store db path as a duplicate db if it exists.
		dbPath := blockDbPath(dbType)
		if fileExists(dbPath) {
			duplicateDbPaths = append(duplicateDbPaths, dbPath)
		}
	}
	// Warn if there are extra databases.
	if len(duplicateDbPaths) > 0 {
		selectedDbPath := blockDbPath(cfg.DbType)
		podLog.Warnf("WARNING: There are multiple block chain databases using different database types.\n"+
			"You probably don't want to waste disk space by having more than one.\n"+
			"Your current database is located at [%v].\n"+
			"The additional database is located at %v", selectedDbPath, duplicateDbPaths)
	}
}

// loadBlockDB loads (or creates when needed) the block database taking into account the selected database backend and returns a handle to it.  It also additional logic such warning the user if there are multiple databases which consume space on the file system and ensuring the regression test database is clean when in regression test mode.
func loadBlockDB() (database.DB, error) {
	// The memdb backend does not have a file path associated with it, so handle it uniquely.  We also don't want to worry about the multiple database type warnings when running with the memory database.
	if cfg.DbType == "memdb" {
		podLog.Infof("Creating block database in memory.")
		db, err := database.Create(cfg.DbType)
		if err != nil {
			return nil, err
		}
		return db, nil
	}
	warnMultipleDBs()
	// The database name is based on the database type.
	dbPath := blockDbPath(cfg.DbType)
	// The regression test is special in that it needs a clean database for each run, so remove it now if it already exists.
	removeRegressionDB(dbPath)
	podLog.Infof("Loading block database from '%s'", dbPath)
	db, err := database.Open(cfg.DbType, dbPath, activeNetParams.Net)
	if err != nil {
		// Return the error if it's not because the database doesn't exist.
		if dbErr, ok := err.(database.Error); !ok || dbErr.ErrorCode !=
			database.ErrDbDoesNotExist {
			return nil, err
		}
		// Create the db if it does not exist.
		err = os.MkdirAll(cfg.DataDir, 0700)
		if err != nil {
			return nil, err
		}
		db, err = database.Create(cfg.DbType, dbPath, activeNetParams.Net)
		if err != nil {
			return nil, err
		}
	}
	podLog.Info("Block database loaded")
	return db, nil
}
