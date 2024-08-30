package s3

import "akvorado/common/reporter"

type metrics struct {
	getObjectSuccess *reporter.CounterVec
	getObjectError   *reporter.CounterVec
}

func (c *Component) initMetrics() {

	c.metrics.getObjectSuccess = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "get_object_success",
			Help: "Number of successful S3 getObject calls",
		}, []string{"bucket", "object"},
	)

	c.metrics.getObjectError = c.r.CounterVec(
		reporter.CounterOpts{
			Name: "get_object_error",
			Help: "Number of failed S3 getObject calls",
		}, []string{"bucket", "object"},
	)

}
