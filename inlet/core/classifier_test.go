// SPDX-FileCopyrightText: 2022 Free Mobile
// SPDX-License-Identifier: AGPL-3.0-only

package core

import (
	"testing"

	"akvorado/common/helpers"
)

func TestExporterClassifier(t *testing.T) {
	cases := []struct {
		Description            string
		Program                string
		ExporterInfo           exporterInfo
		ExpectedClassification exporterClassification
		ExpectedErr            bool
	}{
		{
			Description: "trivial classifier",
			Program:     "false",
		}, {
			Description:            "constant classifier",
			Program:                `Classify("europe")`,
			ExpectedClassification: exporterClassification{Group: "europe"},
		}, {
			Description:            "constant classifier (group)",
			Program:                `ClassifyGroup("europe")`,
			ExpectedClassification: exporterClassification{Group: "europe"},
		}, {
			Description:            "constant classifier (site)",
			Program:                `ClassifySite("paris")`,
			ExpectedClassification: exporterClassification{Site: "paris"},
		}, {
			Description:            "constant classifier (role)",
			Program:                `ClassifyRole("edge")`,
			ExpectedClassification: exporterClassification{Role: "edge"},
		}, {
			Description:            "constant classifier (region)",
			Program:                `ClassifyRegion("europe")`,
			ExpectedClassification: exporterClassification{Region: "europe"},
		}, {
			Description:            "constant classifier (tenant)",
			Program:                `ClassifyTenant("mobile")`,
			ExpectedClassification: exporterClassification{Tenant: "mobile"},
		}, {
			Description:            "access to exporter name",
			Program:                `Exporter.Name startsWith "expo" && Classify("europe")`,
			ExporterInfo:           exporterInfo{"127.0.0.1", "exporter"},
			ExpectedClassification: exporterClassification{Group: "europe"},
		}, {
			Description:            "matches",
			Program:                `Exporter.Name matches "^e.p.r" && Classify("europe")`,
			ExporterInfo:           exporterInfo{"127.0.0.1", "exporter"},
			ExpectedClassification: exporterClassification{Group: "europe"},
		}, {
			Description: "multiline",
			Program: `Exporter.Name matches "^e.p.r" &&
Classify("europe")`,
			ExporterInfo:           exporterInfo{"127.0.0.1", "exporter"},
			ExpectedClassification: exporterClassification{Group: "europe"},
		}, {
			Description:            "regex",
			Program:                `ClassifyRegex(Exporter.Name, "^(e.p+).r", "europe-$1")`,
			ExporterInfo:           exporterInfo{"127.0.0.1", "exporter"},
			ExpectedClassification: exporterClassification{Group: "europe-exp"},
		}, {
			Description:            "regex with class",
			Program:                `ClassifyRegex(Exporter.Name, "^(\\w+).r", "europe-$1")`,
			ExporterInfo:           exporterInfo{"127.0.0.1", "exporter"},
			ExpectedClassification: exporterClassification{Group: "europe-export"},
		}, {
			Description:            "non-matching regex",
			Program:                `ClassifyRegex(Exporter.Name, "^(ebp+).r", "europe-$1")`,
			ExporterInfo:           exporterInfo{"127.0.0.1", "exporter"},
			ExpectedClassification: exporterClassification{Group: ""},
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
		}, {
			Description: "inexistant function",
			Program:     `ClassifyStuff("blip")`,
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
			var classification exporterClassification
			err = scr.exec(tc.ExporterInfo, &classification)
			if !tc.ExpectedErr && err != nil {
				t.Fatalf("exec(%q) error:\n%+v", tc.Program, err)
			}
			if tc.ExpectedErr && err == nil {
				t.Fatalf("exec(%q) no error", tc.Program)
			}
			if diff := helpers.Diff(classification, tc.ExpectedClassification); diff != "" {
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
		ExpectedInterfaceInfo  interfaceInfo
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
			ExpectedInterfaceInfo: interfaceInfo{
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
		{
			Description: "setname test",
			Program: `
			Interface.VLAN != "0" && SetName(Interface.Name + "." + Interface.VLAN)
            `,
			InterfaceInfo: interfaceInfo{
				Name:  "Gi0/0/0",
				Speed: 1000,
				VLAN:  "44",
			},
			ExpectedInterfaceInfo: interfaceInfo{
				Name:  "Gi0/0/0.44",
				Speed: 1000,
				VLAN:  "44",
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
			gotInterfaceInfo := tc.InterfaceInfo
			err = scr.exec(tc.ExporterInfo, &gotInterfaceInfo, &gotClassification)
			if !tc.ExpectedErr && err != nil {
				t.Fatalf("exec(%q) error:\n%+v", tc.Program, err)
			}
			if tc.ExpectedErr && err == nil {
				t.Fatalf("exec(%q) no error", tc.Program)
			}
			if diff := helpers.Diff(gotInterfaceInfo, tc.ExpectedInterfaceInfo); diff != "" {
				t.Fatalf("exec(%q) (-got, +want):\n%s", tc.Program, diff)
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
