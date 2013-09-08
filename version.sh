#!/usr/bin/env bash
echo "package version" > "version/version.ignore.go"
echo "" >> "version/version.ignore.go"
echo "func (v *VersionInfo) Populate() {" >> "version/version.ignore.go"
echo "	v.GitHash = \"`git rev-parse HEAD`\"" >> "version/version.ignore.go"
echo "	v.BuildDate = \"`date`\"" >> "version/version.ignore.go"
echo "}" >> "version/version.ignore.go"