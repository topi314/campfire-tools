package server

import (
	"errors"
	"fmt"
	"html/template"
	"reflect"
	"strings"
	"time"
)

var templateFuncs = template.FuncMap{
	"add":                    add,
	"addStr":                 addStr,
	"seq":                    seq,
	"hasIndex":               hasIndex,
	"now":                    time.Now,
	"dict":                   dict,
	"reverse":                reverse,
	"parseTime":              parseTime,
	"convertNewLinesToBR":    convertNewLinesToBR,
	"safeHTML":               safeHTML,
	"safeCSS":                safeCSS,
	"safeHTMLAttr":           safeHTMLAttr,
	"safeURL":                safeURL,
	"safeJS":                 safeJS,
	"safeJSStr":              safeJSStr,
	"safeSrcset":             safeSrcset,
	"formatDate":             formatDate,
	"formatDateNice":         formatDateNice,
	"formatTimeToDayTime":    formatTimeToDayTime,
	"formatTimeToRelDayTime": formatTimeToRelDayTime,
}

func add(a, b any) (int, error) {
	ai, err := toInt(a)
	if err != nil {
		return 0, fmt.Errorf("first argument must be an integer: %w", err)
	}
	bi, err := toInt(b)
	if err != nil {
		return 0, fmt.Errorf("second argument must be an integer: %w", err)
	}
	return ai + bi, nil
}

func addStr(a ...any) (string, error) {
	var sb strings.Builder
	for _, v := range a {
		s, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("all arguments must be strings: %v", v)
		}
		sb.WriteString(s)
	}
	return sb.String(), nil
}

func seq(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = i
	}
	return s
}

func hasIndex(l any, i any) (bool, error) {
	v, isNil := indirect(reflect.ValueOf(l))
	if isNil {
		return false, nil
	}

	switch v.Kind() {
	case reflect.Array, reflect.Slice:
		ii, err := toInt(i)
		if err != nil {
			return false, err
		}
		return ii >= 0 && ii < v.Len(), nil
	case reflect.Map:
		return v.MapIndex(reflect.ValueOf(i)).IsValid(), nil
	default:
		return false, errors.New("can't check index of " + reflect.ValueOf(l).Type().String())
	}
}

func toInt(a any) (int, error) {
	switch v := a.(type) {
	case int:
		return v, nil
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case uint:
		return int(v), nil
	case uint8:
		return int(v), nil
	case uint16:
		return int(v), nil
	case uint32:
		return int(v), nil
	case uint64:
		return int(v), nil
	}
	return 0, errors.New("can't convert to int")
}

func reverse(l any) (any, error) {
	if l == nil {
		return nil, errors.New("sequence must be provided")
	}

	seqv, isNil := indirect(reflect.ValueOf(l))
	if isNil {
		return nil, errors.New("can't iterate over a nil value")
	}

	var sliceType reflect.Type
	switch seqv.Kind() {
	case reflect.Array, reflect.Slice:
		sliceType = seqv.Type()
	default:
		return nil, errors.New("can't sort " + reflect.ValueOf(l).Type().String())
	}

	length := seqv.Len()
	reversed := reflect.MakeSlice(sliceType, length, length)
	for i := 0; i < length; i++ {
		reversed.Index(i).Set(seqv.Index(length - i - 1))
	}
	return reversed.Interface(), nil
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

func formatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

func formatDateNice(t time.Time) string {
	return t.Format("2 Jan 2006")
}

func formatTimeToDayTime(t time.Time) string {
	return t.Format("Mon _2 Jan 2006 15:04 MST")
}

func formatTimeToRelDayTime(t time.Time) string {
	nowYear, nowMonth, nowDay := time.Now().Date()
	year, month, day := t.Date()

	timeStr := t.Format("15:04 MST")

	switch {
	case year == nowYear && month == nowMonth && day == nowDay:
		return "Today at " + timeStr
	case year == nowYear && month == nowMonth && day == nowDay-1:
		return "Yesterday at " + timeStr
	case year == nowYear && month == nowMonth && day == nowDay+1:
		return "Tomorrow at " + timeStr
	default:
		return formatTimeToDayTime(t)
	}
}

func convertNewLinesToBR(a any) string {
	return strings.ReplaceAll(fmt.Sprint(a), "\n", "<br>")
}

func safeHTML(s string) template.HTML {
	return template.HTML(s)
}

func safeCSS(s string) template.CSS {
	return template.CSS(s)
}

func safeHTMLAttr(s string) template.HTMLAttr {
	return template.HTMLAttr(s)
}

func safeURL(s string) template.URL {
	return template.URL(s)
}

func safeJS(s string) template.JS {
	return template.JS(s)
}

func safeJSStr(s string) template.JSStr {
	return template.JSStr(s)
}

func safeSrcset(s string) template.Srcset {
	return template.Srcset(s)
}

func indirect(v reflect.Value) (rv reflect.Value, isNil bool) {
	for ; v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface; v = v.Elem() {
		if v.IsNil() {
			return v, true
		}
		if v.Kind() == reflect.Interface && v.NumMethod() > 0 {
			break
		}
	}
	return v, false
}

func dict(a ...any) (map[string]any, error) {
	if len(a)%2 != 0 {
		return nil, errors.New("invalid number of arguments, must be even")
	}
	m := make(map[string]any, len(a)/2)
	for i := 0; i < len(a); i += 2 {
		key, ok := a[i].(string)
		if !ok {
			return nil, errors.New("map keys must be strings")
		}
		m[key] = a[i+1]
	}
	return m, nil
}
