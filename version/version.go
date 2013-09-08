package version

import (
	"reflect"
)

var GitHash string
var BuildDate string

type VersionInfo struct {
	GitHash   string
	BuildDate string
}

var versionInfo VersionInfo

func init() {
	ver := reflect.ValueOf(&versionInfo)
	method := ver.MethodByName("Populate")
	if method.IsValid() {
		method.Call(nil)
		GitHash = versionInfo.GitHash
		BuildDate = versionInfo.BuildDate
	} else {
		GitHash = "None Supplied"
		BuildDate = "Unknown"
	}
}

/*
func (v *VersionInfo) Populate() {
  v.GitHash = "POPULATED"
  v.BuildDate = "POPULATED"
}*/
