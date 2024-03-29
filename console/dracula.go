// SPDX-FileCopyrightText: 2016 Rob G <wowmotty@gmail.com>
// SPDX-FileCopyrightText: 2016 Chris Bracco <chris@cbracco.me>
// SPDX-FileCopyrightText: 2016 Zeno Rocha <hi@zenorocha.com>
// SPDX-License-Identifier: MIT

package console

import "github.com/alecthomas/chroma"

// Converted from https://github.com/dracula/pygments/pull/11
var draculaStyle = chroma.MustNewStyle("dracula", chroma.StyleEntries{
	chroma.Comment:              "#6272a4",
	chroma.CommentHashbang:      "#6272a4",
	chroma.CommentMultiline:     "#6272a4",
	chroma.CommentPreproc:       "#ff79c6",
	chroma.CommentPreprocFile:   "#ff79c6",
	chroma.CommentSingle:        "#6272a4",
	chroma.CommentSpecial:       "#8be9fd",
	chroma.Error:                "#ff5555",
	chroma.Generic:              "#ff79c6",
	chroma.GenericDeleted:       "#ff5555",
	chroma.GenericEmph:          "#f1fa8c underline",
	chroma.GenericError:         "#ff5555",
	chroma.GenericHeading:       "#bd93f9 bold",
	chroma.GenericInserted:      "#50fa7b bold",
	chroma.GenericOutput:        "#6272a4",
	chroma.GenericPrompt:        "#50fa7b",
	chroma.GenericStrong:        "#ffb86c",
	chroma.GenericSubheading:    "#bd93f9 bold",
	chroma.GenericTraceback:     "#ff5555",
	chroma.Keyword:              "#ff79c6",
	chroma.KeywordConstant:      "#bd93f9",
	chroma.KeywordDeclaration:   "#ff79c6 italic",
	chroma.KeywordNamespace:     "#ff79c6",
	chroma.KeywordPseudo:        "#ff79c6",
	chroma.KeywordReserved:      "#ff79c6",
	chroma.KeywordType:          "#8be9fd",
	chroma.LineNumbers:          "#6272a4",
	chroma.Literal:              "#ffb86c",
	chroma.LiteralDate:          "#ffb86c",
	chroma.Name:                 "#f8f8f2",
	chroma.NameAttribute:        "#50fa7b",
	chroma.NameBuiltin:          "#bd93f9 italic",
	chroma.NameBuiltinPseudo:    "#bd93f9",
	chroma.NameClass:            "#8be9fd",
	chroma.NameConstant:         "#bd93f9",
	chroma.NameDecorator:        "#50fa7b",
	chroma.NameEntity:           "#ff79c6",
	chroma.NameException:        "#ff5555",
	chroma.NameFunction:         "#50fa7b",
	chroma.NameFunctionMagic:    "#bd93f9",
	chroma.NameLabel:            "#8be9fd italic",
	chroma.NameNamespace:        "#f8f8f2",
	chroma.NameOther:            "#f8f8f2",
	chroma.NameTag:              "#ff79c6",
	chroma.NameVariable:         "#f8f8f2 italic",
	chroma.NameVariableClass:    "#8be9fd italic",
	chroma.NameVariableGlobal:   "#f8f8f2 italic",
	chroma.NameVariableInstance: "#bd93f9 italic",
	chroma.NameVariableMagic:    "#bd93f9",
	chroma.Number:               "#bd93f9",
	chroma.NumberBin:            "#bd93f9",
	chroma.NumberFloat:          "#bd93f9",
	chroma.NumberHex:            "#bd93f9",
	chroma.NumberInteger:        "#bd93f9",
	chroma.NumberIntegerLong:    "#bd93f9",
	chroma.NumberOct:            "#bd93f9",
	chroma.Operator:             "#ff79c6",
	chroma.OperatorWord:         "#ff79c6",
	chroma.Other:                "#f8f8f2",
	chroma.Punctuation:          "#f8f8f2",
	chroma.String:               "#f1fa8c",
	chroma.StringBacktick:       "#50fa7b",
	chroma.StringChar:           "#f1fa8c",
	chroma.StringDoc:            "#f1fa8c",
	chroma.StringDouble:         "#f1fa8c",
	chroma.StringEscape:         "#ff79c6",
	chroma.StringHeredoc:        "#f1fa8c",
	chroma.StringInterpol:       "#ff79c6",
	chroma.StringOther:          "#f1fa8c",
	chroma.StringRegex:          "#ff5555",
	chroma.StringSingle:         "#f1fa8c",
	chroma.StringSymbol:         "#bd93f9",
	chroma.Text:                 "#f8f8f2",
	chroma.Whitespace:           "#f8f8f2",
})
