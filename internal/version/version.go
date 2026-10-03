package version

var (
	Version   = "0.1.0"
	Commit    = "dev"
	BuildDate = "unknown"
)

type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

func Get() Info {
	return Info{Version: Version, Commit: Commit, BuildDate: BuildDate}
}
