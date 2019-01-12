package jdb

import (
	"io/ioutil"
)

type VLibs map[string][]byte

var apphtml, _ = ioutil.ReadFile("./gui/assets/vue/app.html")
var buefyjs, _ = ioutil.ReadFile("./gui/assets/vue/buefy.js")
var buefycss, _ = ioutil.ReadFile("./gui/assets/vue/buefy.css")
var appcss, _ = ioutil.ReadFile("./gui/assets/vue/app.css")
var vue, _ = ioutil.ReadFile("./gui/assets/vue/vue.js")
var easybar, _ = ioutil.ReadFile("./gui/assets/vue/easybar.js")
var comp, _ = ioutil.ReadFile("./gui/assets/vue/comp.js")
var set, _ = ioutil.ReadFile("./gui/assets/vue/settings.js")
var appjs, _ = ioutil.ReadFile("./gui/assets/vue/app.js")

var VLB VLibs = VLibs{
	"apphtml":  apphtml,
	"buefyjs":  buefyjs,
	"buefycss": buefycss,
	"appcss":   appcss,
	"appjs":    appjs,
	"vue":      vue,
	"easybar":  easybar,
	"settings": set,
	"comp":     comp,
}
