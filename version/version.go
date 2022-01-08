package version

var (
	// Version is the main version at the moment.
	Version = "0.1.1"
)

// Versioning should follow the SemVer guidelines
// https://semver.org/

// GetVersion returns a string representation of the version
func GetVersion() string {
	version := "\n[Renloi VERSION]\n"
	version += Version

	version += "\n"

	return version
}
