package clickhouse

import (
	"testing"

	"akvorado/common/helpers"
)

func TestResolutionsToTTL(t *testing.T) {
	resolutions := DefaultConfiguration().Resolutions
	expected := []string{
		"TimeReceived + INTERVAL 21600 second GROUP BY ExporterAddress SET Bytes = SUM(Bytes), Packets = SUM(Packets), TimeReceived = toStartOfInterval(TimeReceived, INTERVAL 10 second)",
		"TimeReceived + INTERVAL 86400 second GROUP BY ExporterAddress SET Bytes = SUM(Bytes), Packets = SUM(Packets), TimeReceived = toStartOfInterval(TimeReceived, INTERVAL 60 second)",
		"TimeReceived + INTERVAL 604800 second GROUP BY ExporterAddress SET Bytes = SUM(Bytes), Packets = SUM(Packets), TimeReceived = toStartOfInterval(TimeReceived, INTERVAL 300 second)",
		"TimeReceived + INTERVAL 7776000 second GROUP BY ExporterAddress SET Bytes = SUM(Bytes), Packets = SUM(Packets), TimeReceived = toStartOfInterval(TimeReceived, INTERVAL 3600 second)",
		"TimeReceived + INTERVAL 15552000 second DELETE",
	}
	got := resolutionsToTTL(resolutions, "ExporterAddress")
	if diff := helpers.Diff(got, expected); diff != "" {
		t.Fatalf("resolutiosnToTTL() (-got, +want):\n%s", diff)
	}
}
