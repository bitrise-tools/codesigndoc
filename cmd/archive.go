package cmd

import (
	"github.com/bitrise-tools/go-xcode/plistutil"
	"github.com/bitrise-tools/go-xcode/profileutil"
)

// Archive ...
type Archive interface {
	BundleIDEntitlementsMap() map[string]plistutil.PlistData
	IsXcodeManaged() bool
	SigningIdentity() string
	BundleIDProfileInfoMap() map[string]profileutil.ProvisioningProfileInfoModel
}
