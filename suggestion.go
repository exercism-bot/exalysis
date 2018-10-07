package exalysis

import (
	"fmt"
	"github.com/tehsphinx/exalysis/track/raindrops"
	"go/format"
	"log"
	"strings"

	"github.com/golang/lint"
	"github.com/pmezard/go-difflib/difflib"
	"github.com/tehsphinx/astrav"
	"github.com/tehsphinx/dbg"
	"github.com/tehsphinx/exalysis/extypes"
	"github.com/tehsphinx/exalysis/gtpl"
	"github.com/tehsphinx/exalysis/track/hamming"
	"github.com/tehsphinx/exalysis/track/scrabble"
	"github.com/tehsphinx/exalysis/track/twofer"
)

var (
	//LintMinConfidence sets the min confidence for linting
	LintMinConfidence float64
)

var exercisePkgs = map[string]extypes.SuggestionFunc{
	"twofer":   twofer.Suggest,
	"hamming":  hamming.Suggest,
	"raindrops":  raindrops.Suggest,
	"scrabble": scrabble.Suggest,
}

//GetSuggestions selects the package suggestion routine and returns the suggestions
func GetSuggestions() (string, string) {
	folder := astrav.NewFolder(".")
	_, err := folder.ParseFolder()
	if err != nil {
		log.Fatal(err)
	}

	pkg, suggFunc := getExercisePkg(folder)
	if suggFunc == nil {
		log.Fatal("no known exercise package found or not implemented")
	}

	var r = extypes.NewResponse()
	addGreeting(r, pkg.Name, folder.StudentName())

	files := folder.GetRawFiles()
	fmtOk := checkFmt(files, r, pkg.Name)
	lintOk := checkLint(files, r, pkg.Name)

	suggFunc(pkg, r)

	return r.GetAnswerString(), approval(r, fmtOk, lintOk)
}

func checkFmt(files map[string][]byte, r *extypes.Response, pkgName string) bool {
	resFmt := fmtCode(files)
	if resFmt == "" {
		return true
	}

	dbg.Cyan("#### gofmt")
	fmt.Println(resFmt)
	if pkgName == "twofer" || pkgName == "hamming" {
		r.AppendImprovement(gtpl.NotFormatted)
		return true
	} else {
		r.AppendTodo(gtpl.NotFormatted)
	}

	return false
}

func checkLint(files map[string][]byte, r *extypes.Response, pkgName string) bool {
	resLint := lintCode(files)
	if resLint == "" {
		return true
	}

	dbg.Cyan("#### golint")
	fmt.Println(resLint)
	if pkgName == "twofer" || pkgName == "hamming" {
		r.AppendImprovement(gtpl.NotLinted)
		return true
	} else {
		r.AppendTodo(gtpl.NotLinted)
	}

	return false
}

func lintCode(files map[string][]byte) string {
	l := lint.Linter{}
	ps, err := l.LintFiles(files)
	if err != nil {
		log.Fatal(err)
	}

	var lintRes string
	for _, p := range ps {
		if p.Confidence < LintMinConfidence {
			continue
		}
		lintRes += fmt.Sprintf("%s: %s\n\t%s\n\tdoc: %s\n", p.Category, p.Text, p.Position.String(), p.Link)
	}
	return lintRes
}

func fmtCode(files map[string][]byte) string {
	for _, file := range files {
		f, err := format.Source(file)
		if err != nil {
			return fmt.Sprintf("code fails to format with error: %s\n", err)
		}
		if string(f) != strings.Replace(string(file), "\r\n", "\n", -1) {
			return getDiff(file, f)
		}
	}
	return ""
}

func getDiff(current, formatted []byte) string {
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(current)),
		B:        difflib.SplitLines(string(formatted)),
		FromFile: "Current",
		ToFile:   "Formatted",
		Context:  0,
	}
	text, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return fmt.Sprintf("error while diffing strings: %s", err)
	}
	return text
}

func getExercisePkg(folder *astrav.Folder) (*astrav.Package, extypes.SuggestionFunc) {
	for name, pkg := range folder.Pkgs {
		if sg, ok := exercisePkgs[name]; ok {
			return pkg, sg
		}
	}
	return nil, nil
}

func addGreeting(r *extypes.Response, pkg, student string) {
	r.SetGreeting(gtpl.Greeting.Format(student))
	switch pkg {
	case "twofer":
		r.AppendGreeting(gtpl.NewcomerGreeting)
	}
}

func approval(r *extypes.Response, gofmt, golint bool) string {
	rating := "\n\n" + dbg.Sprint(dbg.ColorCyan, "#### Rating Suggestion")
	var approve string
	if !gofmt {
		rating += dbg.Sprint(dbg.ColorRed, "go code not formatted")
		if approve == "" {
			approve = dbg.Sprint(dbg.ColorRed, "NO APPROVAL")
		}
	}
	if !golint {
		rating += dbg.Sprint(dbg.ColorRed, "go code not linted")
		if approve == "" {
			approve = dbg.Sprint(dbg.ColorRed, "NO APPROVAL")
		}
	}

	l := r.LenSuggestions()
	var suggsAdded = fmt.Sprintf("Suggestions added: %d", l)
	if 5 < l {
		rating += dbg.Sprint(dbg.ColorRed, suggsAdded)
		if approve == "" {
			approve = dbg.Sprint(dbg.ColorRed, "NO APPROVAL")
		}
	} else if 2 < l {
		rating += dbg.Sprint(dbg.ColorMagenta, suggsAdded)
		if approve == "" {
			approve = dbg.Sprint(dbg.ColorMagenta, "MAYBE APPROVE")
		}
	} else if 1 < l {
		rating += dbg.Sprint(dbg.ColorYellow, suggsAdded)
		if approve == "" {
			approve = dbg.Sprint(dbg.ColorYellow, "LIKELY APPROVE")
		}
	}
	if approve == "" {
		approve = dbg.Sprint(dbg.ColorGreen, "APPROVE")
	}
	rating += "Suggestion: " + approve
	return rating
}
