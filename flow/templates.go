package flow

import (
	"strconv"

	"github.com/netsampler/goflow2/decoders/netflow"
)

type templateSystem struct {
	c         *Component
	key       string
	templates *netflow.BasicTemplateSystem
}

func (s *templateSystem) AddTemplate(version uint16, obsDomainID uint32, template interface{}) {
	s.templates.AddTemplate(version, obsDomainID, template)

	typeStr := "options_template"
	var templateID uint16
	switch templateIDConv := template.(type) {
	case netflow.IPFIXOptionsTemplateRecord:
		templateID = templateIDConv.TemplateId
	case netflow.NFv9OptionsTemplateRecord:
		templateID = templateIDConv.TemplateId
	case netflow.TemplateRecord:
		templateID = templateIDConv.TemplateId
		typeStr = "template"
	}

	s.c.metrics.netflowTemplatesStats.WithLabelValues(
		s.key,
		strconv.Itoa(int(version)),
		strconv.Itoa(int(obsDomainID)),
		strconv.Itoa(int(templateID)),
		typeStr,
	).
		Inc()
}

func (s *templateSystem) GetTemplate(version uint16, obsDomainID uint32, templateID uint16) (interface{}, error) {
	return s.templates.GetTemplate(version, obsDomainID, templateID)
}
