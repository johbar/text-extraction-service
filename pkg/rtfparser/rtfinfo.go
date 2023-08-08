package rtfparser

import (
	"errors"
	"strconv"
	"time"

	"github.com/dlclark/regexp2"
)

type RtfMetadata struct {
	Author, Comment, Company, Category, Operator, Subject, Title string
	// Created and last modified times have no timezone information attached in RTF files.
	// This packages returns them as local times, effectively attaching the running systems timezone information.
	Created, Modified time.Time
}

var nul = regexp2.None
var infoGroup = regexp2.MustCompile(`(?s)\{\\info.+?\{.+?(?<!\\)\}{2}`, nul)
var author = regexp2.MustCompile(`\\author (.*?)(?<!\\)\}`, nul)
var category = regexp2.MustCompile(`\\category (.*?)(?<!\\)\}`, nul)
var comment = regexp2.MustCompile(`\{\\doccomm (.*?)(?<!\\)\}`, nul)
var company = regexp2.MustCompile(`\{\\company (.*?)(?<!\\)\}`, nul)
var operator = regexp2.MustCompile(`\{\\operator (.*?)(?<!\\)\}`, nul)
var title = regexp2.MustCompile(`\{\\title (.*?)(?<!\\)\}`, nul)
var subject = regexp2.MustCompile(`\{\\subject (.*?)(?<!\\)\}`, nul)
var created = regexp2.MustCompile(`\{\\creatim\\yr(?<year>\d{4})\\mo(?<month>\d{1,2})\\dy(?<day>\d{1,2})\\hr(?<hour>\d{1,2})*\\min(?<minute>\d{1,2})(?<!\\)\}`, nul)
var modified = regexp2.MustCompile(`\{\\revtim\\yr(?<year>\d{4})\\mo(?<month>\d{1,2})\\dy(?<day>\d{1,2})\\hr(?<hour>\d{1,2})*\\min(?<minute>\d{1,2})(?<!\\)\}`, nul)

// GetRtfInfo extracts some metadata from the RTF input string.
func GetRtfInfo(inputRtf string) (m RtfMetadata, err error) {
	infoMatch, err := infoGroup.FindStringMatch(inputRtf)
	if err != nil || infoMatch == nil {
		err = errors.New("getInfo: failed to find any RTF metadata")
		return
	}
	info := infoMatch.String()
	if matches, _ := author.FindStringMatch(info); matches != nil && matches.GroupCount() > 0 {
		m.Author = decodeGroup1(matches)
	}
	if matches, _ := category.FindStringMatch(info); matches != nil && matches.GroupCount() > 0 {
		m.Category = decodeGroup1(matches)
	}
	if matches, _ := comment.FindStringMatch(info); matches != nil && matches.GroupCount() > 0 {
		m.Comment = decodeGroup1(matches)
	}
	if matches, _ := company.FindStringMatch(info); matches != nil && matches.GroupCount() > 0 {
		m.Company = decodeGroup1(matches)
	}
	if matches, _ := operator.FindStringMatch(info); matches != nil && matches.GroupCount() > 0 {
		m.Operator = decodeGroup1(matches)
	}
	if matches, _ := subject.FindStringMatch(inputRtf); matches != nil && matches.GroupCount() > 0 {
		m.Subject = decodeGroup1(matches)
	}
	if matches, _ := title.FindStringMatch(inputRtf); matches != nil && matches.GroupCount() > 0 {
		m.Title = decodeGroup1(matches)
	}
	if matches, _ := created.FindStringMatch(inputRtf); matches != nil && matches.GroupCount() > 0 {
		m.Created, err = parseDate(matches)
	}
	if matches, _ := modified.FindStringMatch(inputRtf); matches != nil && matches.GroupCount() > 0 {
		m.Modified, err = parseDate(matches)
	}
	return
}

func parseDate(match *regexp2.Match) (date time.Time, err error) {
	m := map[string]int{}
	for _, x := range []string{"year", "month", "day", "hour", "minute"} {
		if tmp, err := strconv.Atoi(match.GroupByName(x).String()); err == nil {
			m[x] = tmp
		}
		date = time.Date(m["year"], (time.Month)(m["month"]), m["day"], m["hour"], m["minute"], 0, 0, time.Local)
	}
	return
}

// decodeGroup1 translates all RTF-encoded chars of the first capture group
// an returns a readable string
func decodeGroup1(matches *regexp2.Match) string {
	return Rtf2Text(matches.GroupByNumber(1).String())
}
