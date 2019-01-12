package jdb

type VSettings struct {
	InfSet InfSet `json:"inf"`
	NetSet NetSet `json:"net"`
	SecSet SecSet `json:"sec"`
	Mining Mining `json:"mining"`
}
type InfSet struct {
	Lang  string `json:"lang"`
	Deno  string `json:"deno"`
	Fiat  string `json:"fiat"`
	Theme string `json:"theme"`
	CCSS  string `json:"ccss"`
	Start string `json:"start"`
	Tray  bool   `json:"tray"`
}

type NetSet struct {
	TLS     bool   `json:"tls"`
	Network string `json:"network"`
	RPC     string `json:"rpc"`
	SRPC    string `json:"srpc"`
	TLSpub  string `json:"tlspub"`
	TLSpri  string `json:"tlspri"`
	Proxy   string `json:"rpc"`
}

type SecSet struct {
	Network string `json:"network"`
}

type Mining struct {
	Algo  string `json:"algo"`
	CPU   string `json:"cpu"`
	Cores uint   `json:"cores"`
}

// var appHtml, _ = ioutil.ReadFile("./assets/vue/app.html")

var VST VSettings = VSettings{
	// "apphtml": appHtml,
}
