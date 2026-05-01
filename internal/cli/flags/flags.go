package flags

import "github.com/bwilczynski/hlctl/internal/output"

var (
	OutputFormat string
	APIURL       string
)

func GetOutputFormat() output.Format {
	return output.Format(OutputFormat)
}

func GetAPIURL() string {
	return APIURL
}
