package core

import (
	"testing"

	"akvorado/helpers"
)

func TestExporterClassifier(t *testing.T) {
	cases := []struct {
		Description   string
		Program       string
		ExporterInfo  exporterInfo
		ExpectedGroup string
		ExpectedErr   bool
	}{
		{
			Description: "trivial classifier",
			Program:     "false",
		}, {
			Description:   "constant classifier",
			Program:       `Classify("europe")`,
			ExpectedGroup: "europe",
		}, {
			Description:   "access to exporter name",
			Program:       `Exporter.Name startsWith "expo" && Classify("europe")`,
			ExporterInfo:  exporterInfo{"127.0.0.1", "exporter"},
			ExpectedGroup: "europe",
		}, {
			Description:   "matches",
			Program:       `Exporter.Name matches "^e.p.r" && Classify("europe")`,
			ExporterInfo:  exporterInfo{"127.0.0.1", "exporter"},
			ExpectedGroup: "europe",
		}, {
			Description: "multiline",
			Program: `Exporter.Name matches "^e.p.r" &&
Classify("europe")`,
			ExporterInfo:  exporterInfo{"127.0.0.1", "exporter"},
			ExpectedGroup: "europe",
		}, {
			Description:   "regex",
			Program:       `ClassifyRegex(Exporter.Name, "^(e.p+).r", "europe-$1")`,
			ExporterInfo:  exporterInfo{"127.0.0.1", "exporter"},
			ExpectedGroup: "europe-exp",
		}, {
			Description:   "non-matching regex",
			Program:       `ClassifyRegex(Exporter.Name, "^(ebp+).r", "europe-$1")`,
			ExporterInfo:  exporterInfo{"127.0.0.1", "exporter"},
			ExpectedGroup: "",
		}, {
			Description:  "faulty regex",
			Program:      `ClassifyRegex(Exporter.Name, "^(ebp+.r", "europe-$1")`,
			ExporterInfo: exporterInfo{"127.0.0.1", "exporter"},
			ExpectedErr:  true,
		}, {
			Description: "syntax error",
			Program:     `Classify("europe"`,
			ExpectedErr: true,
		}, {
			Description: "incorrect typing",
			Program:     `Classify(1)`,
			ExpectedErr: true,
		}, {
			Description: "another incorrect typing",
			Program:     `"hello"`,
			ExpectedErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			var scr ExporterClassifierRule
			err := scr.UnmarshalText([]byte(tc.Program))
			if !tc.ExpectedErr && err != nil {
				t.Fatalf("UnmarshalText(%q) error:\n%+v", tc.Program, err)
			}
			if tc.ExpectedErr && err != nil {
				return
			}
			group, err := scr.exec(tc.ExporterInfo)
			if !tc.ExpectedErr && err != nil {
				t.Fatalf("exec(%q) error:\n%+v", tc.Program, err)
			}
			if tc.ExpectedErr && err == nil {
				t.Fatalf("exec(%q) no error", tc.Program)
			}
			if diff := helpers.Diff(group, tc.ExpectedGroup); diff != "" {
				t.Fatalf("exec(%q) (-got, +want):\n%s", tc.Program, diff)
			}
		})
	}
}

func TestInterfaceClassifier(t *testing.T) {
	cases := []struct {
		Description            string
		Program                string
		ExporterInfo           exporterInfo
		InterfaceInfo          interfaceInfo
		ExpectedClassification interfaceClassification
		ExpectedErr            bool
	}{
		{
			Description: "trivial classifier",
			Program:     "false",
		}, {
			Description:            "constant classifier for connectivity",
			Program:                `ClassifyConnectivity("Transit")`,
			ExpectedClassification: interfaceClassification{Connectivity: "transit"},
		}, {
			Description:            "constant classifier for provider",
			Program:                `ClassifyProvider("Telia")`,
			ExpectedClassification: interfaceClassification{Provider: "telia"},
		}, {
			Description:            "constant classifier for boundary external",
			Program:                `ClassifyExternal()`,
			ExpectedClassification: interfaceClassification{Boundary: externalBoundary},
		}, {
			Description:            "constant classifier for boundary internal",
			Program:                `ClassifyInternal()`,
			ExpectedClassification: interfaceClassification{Boundary: internalBoundary},
		}, {
			Description: "complex example",
			Program: `
Interface.Description startsWith "Transit:" &&
ClassifyConnectivity("transit") &&
ClassifyExternal() &&
ClassifyProviderRegex(Interface.Description, "^Transit: ([^ ]+)", "$1")
`,
			InterfaceInfo: interfaceInfo{
				Name:        "Gi0/0/0",
				Description: "Transit: Telia (GWDM something something)",
				Speed:       1000,
			},
			ExpectedClassification: interfaceClassification{
				Connectivity: "transit",
				Provider:     "telia",
				Boundary:     externalBoundary,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.Description, func(t *testing.T) {
			var scr InterfaceClassifierRule
			err := scr.UnmarshalText([]byte(tc.Program))
			if !tc.ExpectedErr && err != nil {
				t.Fatalf("UnmarshalText(%q) error:\n%+v", tc.Program, err)
			}
			if tc.ExpectedErr && err != nil {
				return
			}
			var gotClassification interfaceClassification
			err = scr.exec(tc.ExporterInfo, tc.InterfaceInfo, &gotClassification)
			if !tc.ExpectedErr && err != nil {
				t.Fatalf("exec(%q) error:\n%+v", tc.Program, err)
			}
			if tc.ExpectedErr && err == nil {
				t.Fatalf("exec(%q) no error", tc.Program)
			}
			if diff := helpers.Diff(gotClassification, tc.ExpectedClassification); diff != "" {
				t.Fatalf("exec(%q) (-got, +want):\n%s", tc.Program, diff)
			}
		})
	}
}

func TestRegexValidation(t *testing.T) {
	cases := []struct {
		Classifier string
		Error      bool
	}{
		{`ClassifyRegex("something", "^(ebp+).r", "europe-$1")`, false},
		{`ClassifyRegex("something", "^(ebp+.r", "europe-$1")`, true},
		// When non-constant string is used, we cannot detect the error
		{`ClassifyRegex("something", Exporter.Name + "^(ebp+.r", "europe-$1")`, false},
	}
	for _, tc := range cases {
		var scr ExporterClassifierRule
		err := scr.UnmarshalText([]byte(tc.Classifier))
		if err == nil && tc.Error {
			t.Errorf("UnmarshalText(%q) should have returned an error", tc.Classifier)
		}
		if err != nil && !tc.Error {
			t.Errorf("UnmarshalText(%q) error:\n%+v", tc.Classifier, err)
		}
	}
}
