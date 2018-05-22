// Go support for leveled logs, analogous to https://code.google.com/p/google-glog/
//
// Copyright 2013 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Json for logs.

package glog

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
	"unsafe"
)

// Version is the `version' value of each log entry.
var Version string

// FieldPrefix to add prefix to custom field.
var FieldPrefix string

// EscapeHTML specifies whether problematic HTML characters should be escaped inside JSON quoted strings.
var EscapeHTML bool

// Struct is a struct type that implements Infos (like zerolog/zap) etc. See the documentation of S for more information.
type Struct struct {
	data  [1024]byte
	buf   []byte
	json  bool
	sev   severity
	level Level
	file  string
	line  int
}

// S is a structed logging like V
//	glog.S(2).Int("answer", 42).Msg("log this")
//
// Whether an individual call to S generates a log record depends on the setting of
// the -v and --vmodule flags; both are off by default. If the level in the call to
// S is at least the value of -v, or of -vmodule for the source file containing the
// call, the S call will log.
func S(level Level) *Struct {
	return newStruct(2, infoLog, level)
}

// Info is equivalent to S(0).
// See the documentation of V for usage.
func Infos() *Struct {
	return newStruct(2, infoLog, 0)
}

// Info is equivalent to S(0), but use warning level logger.
// See the documentation of V for usage.
func Warnings() *Struct {
	return newStruct(2, warningLog, 0)
}

// Info is equivalent to S(0), but use error level logger.
// See the documentation of V for usage.
func Errors() *Struct {
	return newStruct(2, errorLog, 0)
}

// Info is equivalent to S(0), but use fatal level logger.
// See the documentation of V for usage.
func Fatals() *Struct {
	return newStruct(2, fatalLog, 0)
}

// Int adds the field name with i as a int to the logger *Struct.
func (s *Struct) Int(name string, i int) *Struct {
	return s.Int64(name, int64(i))
}

// Int8 adds the field name with i as a int8 to the logger *Struct.
func (s *Struct) Int8(name string, i int8) *Struct {
	return s.Int64(name, int64(i))
}

// Int16 adds the field name with i as a int16 to the logger *Struct.
func (s *Struct) Int16(name string, i int16) *Struct {
	return s.Int64(name, int64(i))
}

// Int32 adds the field name with i as a int32 to the logger *Struct.
func (s *Struct) Int32(name string, i int32) *Struct {
	return s.Int64(name, int64(i))
}

// Uint adds the field name with i as a uint to the logger *Struct.
func (s *Struct) Uint(name string, i uint) *Struct {
	return s.Uint64(name, uint64(i))
}

// Uint8 adds the field name with i as a uint8 to the logger *Struct.
func (s *Struct) Uint8(name string, i uint8) *Struct {
	return s.Uint64(name, uint64(i))
}

// Uint16 adds the field name with i as a uint16 to the logger *Struct.
func (s *Struct) Uint16(name string, i uint16) *Struct {
	return s.Uint64(name, uint64(i))
}

// Uint32 adds the field name with i as a uint32 to the logger *Struct.
func (s *Struct) Uint32(name string, i uint32) *Struct {
	return s.Uint64(name, uint64(i))
}

func (s *Struct) appendName(name string) {
	if s.json {
		s.buf = append(s.buf, ',', '"')
		if FieldPrefix != "" {
			s.buf = append(s.buf, FieldPrefix...)
		}
		s.buf = append(s.buf, name...)
		s.buf = append(s.buf, '"', ':')
	} else {
		s.buf = append(s.buf, ' ')
		if FieldPrefix != "" {
			s.buf = append(s.buf, FieldPrefix...)
		}
		s.buf = append(s.buf, name...)
		s.buf = append(s.buf, '=')
	}
}

// Int64 adds the field name with i as a int64 to the logger *Struct.
func (s *Struct) Int64(name string, i int64) *Struct {
	if s != nil {
		s.appendName(name)
		s.buf = strconv.AppendInt(s.buf, i, 10)
	}
	return s
}

// Uint64 adds the field name with i as a uint64 to the logger *Struct.
func (s *Struct) Uint64(name string, i uint64) *Struct {
	if s != nil {
		s.appendName(name)
		s.buf = strconv.AppendUint(s.buf, i, 10)
	}
	return s
}

// Float32 adds the field name with i as a float32 to the logger *Struct.
func (s *Struct) Float32(name string, f float32) *Struct {
	if s != nil {
		s.appendName(name)
		s.buf = strconv.AppendFloat(s.buf, float64(f), 'f', -1, 64)
	}
	return s
}

// Float64 adds the field name with i as a float64 to the logger *Struct.
func (s *Struct) Float64(name string, f float64) *Struct {
	if s != nil {
		s.appendName(name)
		s.buf = strconv.AppendFloat(s.buf, f, 'f', -1, 64)
	}
	return s
}

// Bool adds the field key with val as a bool to the logger *Struct.
func (s *Struct) Bool(name string, b bool) *Struct {
	if s != nil {
		s.appendName(name)
		s.buf = strconv.AppendBool(s.buf, b)
	}
	return s
}

// Err adds the field "error" with err as a string to the logger *Struct. If err is nil, no field is added.
func (s *Struct) Err(err error) *Struct {
	if s != nil && err != nil {
		if s.json {
			s.buf = append(s.buf, ",\"error\":"...)
		} else {
			s.buf = append(s.buf, " error="...)
		}
		s.string(err.Error(), EscapeHTML)
	}
	return s
}

// Str adds the field name with value as a string to the logger *Struct.
func (s *Struct) Str(name string, value string) *Struct {
	if s != nil {
		s.appendName(name)
		s.string(value, EscapeHTML)
	}
	return s
}

// Bytes adds the field name with b as a string to the logger *Struct.
func (s *Struct) Bytes(name string, b []byte) *Struct {
	if s != nil {
		s.appendName(name)
		s.string(*(*string)(unsafe.Pointer(&b)), EscapeHTML)
	}
	return s
}

// Strs adds the field name with values as a []string to the logger *Struct.
func (s *Struct) Strs(name string, values []string) *Struct {
	if s != nil {
		s.appendName(name)
		s.buf = append(s.buf, '[')
		for i, v := range values {
			if i != 0 {
				s.buf = append(s.buf, ',')
			}
			s.string(v, EscapeHTML)
		}
		s.buf = append(s.buf, ']')
	}
	return s
}

// Msg sends the logger with msg added as the message field if not empty.
//
// NOTICE: once this method is called, the logger *Struct should be disposed. Calling Msg twice can have unexpected result.
func (s *Struct) Msg(msg string) {
	if s != nil {
		if s.json {
			if msg != "" {
				s.buf = append(s.buf, ",\"message\":"...)
				s.string(msg, false)
			}
			s.buf = append(s.buf, '}', '\n')
		} else {
			if msg != "" {
				s.buf = append(s.buf, " message="...)
				s.string(msg, false)
			}
			s.buf = append(s.buf, '\n')
		}
		logging.outputs(s.sev, s, s.file, s.line, false)
	}
}

// Msgf sends the logger with formated msg added as the message field if not empty.
//
// NOTICE: once this methid is called, the logger *Struct should be disposed. Calling Msg twice can have unexpected result.
func (s *Struct) Msgf(format string, v ...interface{}) {
	if s != nil {
		s.Msg(fmt.Sprintf(format, v...))
	}
}

func newStruct(depth int, s severity, level Level) *Struct {
	_, file, line, ok := runtime.Caller(depth)

	if !ok {
		file = "???"
		line = 1
	} else {
		slash := strings.LastIndex(file, "/")
		if slash >= 0 {
			file = file[slash+1:]
		}
	}

	if line < 0 {
		line = 0 // not a real line number, but acceptable to someDigits
	}

	if logging.verbosity.get() >= level {
		return logging.structHeader(s, level, file, line)
	}

	if atomic.LoadInt32(&logging.filterLength) > 0 {
		// Now we need a proper lock to use the logging structure. The pcs field
		// is shared so we must lock before accessing it. This is fairly expensive,
		// but if V logging is enabled we're slow anyway.
		logging.mu.Lock()
		defer logging.mu.Unlock()
		if runtime.Callers(2, logging.pcs[:]) == 0 {
			return nil
		}
		v, ok := logging.vmap[logging.pcs[0]]
		if !ok {
			v = logging.setV(logging.pcs[0])
		}

		if v >= level {
			return logging.structHeader(s, level, file, line)
		}
	}

	return nil
}

var spool = sync.Pool{
	New: func() interface{} {
		return new(Struct)
	},
}

func (log *loggingT) structHeader(sev severity, level Level, file string, line int) *Struct {
	now := timeNow()

	s := spool.Get().(*Struct)
	s.json = !log.toStderr || !isatty
	s.sev = sev
	s.level = level
	s.file = file
	s.line = line
	s.buf = s.data[:0]

	if s.json {
		// {"time":"2016-01-02T15:04:05.999999+07:00","level":"info",
		s.buf = append(s.buf, "{\"time\":\""...)
		s.buf = now.AppendFormat(s.buf, "2006-01-02T15:04:05.999999Z07:00")
		s.buf = append(s.buf, "\",\"severity\":\""...)
		s.buf = append(s.buf, lowerSeverityName[sev]...)
		s.buf = append(s.buf, "\",\"level\":"...)
		s.buf = strconv.AppendInt(s.buf, int64(s.level), 10)
		s.buf = append(s.buf, ",\"host\":\""...)
		s.buf = append(s.buf, host...)
		if Version != "" {
			s.buf = append(s.buf, "\",\"version\":\""...)
			s.buf = append(s.buf, Version...)
		}
		s.buf = append(s.buf, "\",\"pid\":"...)
		s.buf = strconv.AppendInt(s.buf, int64(pid), 10)
		s.buf = append(s.buf, ",\"file\":\""...)
		s.buf = append(s.buf, file...)
		s.buf = append(s.buf, "\",\"line\":"...)
		s.buf = strconv.AppendInt(s.buf, int64(line), 10)
	} else {
		// Lmmdd hh:mm:ss.uuuuuu threadid file:line]
		s.buf = append(s.buf, severityChar[sev])
		s.buf = now.AppendFormat(s.buf, "0102 15:04:05.000000 ")
		s.buf = strconv.AppendInt(s.buf, int64(pid), 10)
		s.buf = append(s.buf, ' ')
		s.buf = append(s.buf, file...)
		s.buf = append(s.buf, ':')
		s.buf = strconv.AppendInt(s.buf, int64(line), 10)
		s.buf = append(s.buf, ']')
	}

	return s
}

func (l *loggingT) outputs(s severity, buf *Struct, file string, line int, alsoToStderr bool) {
	l.mu.Lock()
	if l.traceLocation.isSet() {
		if l.traceLocation.match(file, line) {
			buf.buf = append(buf.buf, stacks(false)...)
		}
	}
	data := buf.buf
	if !flag.Parsed() {
		os.Stderr.Write([]byte("ERROR: logging before flag.Parse: "))
		os.Stderr.Write(data)
	} else if l.toStderr {
		if buf.json {
			os.Stderr.Write(data)
		} else {
			WriteFileWithColor(os.Stderr, data, s)
		}
	} else {
		if alsoToStderr || l.alsoToStderr || s >= l.stderrThreshold.get() {
			os.Stderr.Write(data)
		}
		if l.file[s] == nil {
			if err := l.createFiles(s); err != nil {
				os.Stderr.Write(data) // Make sure the message appears somewhere.
				l.exit(err)
			}
		}
		switch s {
		case fatalLog:
			l.file[fatalLog].Write(data)
			fallthrough
		case errorLog:
			l.file[errorLog].Write(data)
			fallthrough
		case warningLog:
			l.file[warningLog].Write(data)
			fallthrough
		case infoLog:
			l.file[infoLog].Write(data)
		}
	}
	if s == fatalLog {
		// If we got here via Exit rather than Fatal, print no stacks.
		if atomic.LoadUint32(&fatalNoStacks) > 0 {
			l.mu.Unlock()
			timeoutFlush(10 * time.Second)
			os.Exit(1)
		}
		// Dump all goroutine stacks before exiting.
		// First, make sure we see the trace for the current goroutine on standard error.
		// If -logtostderr has been specified, the loop below will do that anyway
		// as the first stack in the full dump.
		if !l.toStderr {
			os.Stderr.Write(stacks(false))
		}
		// Write the stack trace for all goroutines to the files.
		trace := stacks(true)
		logExitFunc = func(error) {} // If we get a write error, we'll still exit below.
		for log := fatalLog; log >= infoLog; log-- {
			if f := l.file[log]; f != nil { // Can be nil if -logtostderr is set.
				f.Write(trace)
			}
		}
		l.mu.Unlock()
		timeoutFlush(10 * time.Second)
		os.Exit(255) // C++ uses -1, which is silly because it's anded with 255 anyway.
	}
	spool.Put(buf)
	l.mu.Unlock()
	if stats := severityStats[s]; stats != nil {
		atomic.AddInt64(&stats.lines, 1)
		atomic.AddInt64(&stats.bytes, int64(len(data)))
	}
}

var hex = "0123456789abcdef"

// https://golang.org/src/encoding/json/encode.go
func (s *Struct) string(value string, escapeHTML bool) {
	s.buf = append(s.buf, '"')
	start := 0
	for i := 0; i < len(value); {
		if b := value[i]; b < utf8.RuneSelf {
			if htmlSafeSet[b] || (!escapeHTML && safeSet[b]) {
				i++
				continue
			}
			if start < i {
				s.buf = append(s.buf, value[start:i]...)
			}
			switch b {
			case '\\', '"':
				s.buf = append(s.buf, '\\', b)
			case '\n':
				s.buf = append(s.buf, '\\', 'n')
			case '\r':
				s.buf = append(s.buf, '\\', 'r')
			case '\t':
				s.buf = append(s.buf, '\\', 't')
			default:
				// This encodes bytes < 0x20 except for \t, \n and \r.
				// If escapeHTML is set, it also escapes <, >, and &
				// because they can lead to security holes when
				// user-controlled strings are rendered into JSON
				// and served to some browsers.
				s.buf = append(s.buf, '\\', 'u', '0', '0', hex[b>>4], hex[b&0xF])
			}
			i++
			start = i
			continue
		}
		c, size := utf8.DecodeRuneInString(value[i:])
		if c == utf8.RuneError && size == 1 {
			if start < i {
				s.buf = append(s.buf, value[start:i]...)
			}
			s.buf = append(s.buf, '\\', 'u', 'f', 'f', 'f', 'd')
			i += size
			start = i
			continue
		}
		// U+2028 is LINE SEPARATOR.
		// U+2029 is PARAGRAPH SEPARATOR.
		// They are both technically valid characters in JSON strings,
		// but don't work in JSONP, which has to be evaluated as JavaScript,
		// and can lead to security holes there. It is valid JSON to
		// escape them, so we do so unconditionally.
		// See http://timelessrepo.com/json-isnt-a-javascript-subset for discussion.
		if c == '\u2028' || c == '\u2029' {
			if start < i {
				s.buf = append(s.buf, value[start:i]...)
			}
			s.buf = append(s.buf, '\\', 'u', '2', '0', '2', hex[c&0xF])
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(value) {
		s.buf = append(s.buf, value[start:]...)
	}
	s.buf = append(s.buf, '"')
}

var lowerSeverityName = []string{
	infoLog:    "info",
	warningLog: "warning",
	errorLog:   "error",
	fatalLog:   "fatal",
}

// safeSet holds the value true if the ASCII character with the given array
// position can be represented inside a JSON string without any further
// escaping.
//
// All values are true except for the ASCII control characters (0-31), the
// double quote ("), and the backslash character ("\").
var safeSet = [utf8.RuneSelf]bool{
	' ':      true,
	'!':      true,
	'"':      false,
	'#':      true,
	'$':      true,
	'%':      true,
	'&':      true,
	'\'':     true,
	'(':      true,
	')':      true,
	'*':      true,
	'+':      true,
	',':      true,
	'-':      true,
	'.':      true,
	'/':      true,
	'0':      true,
	'1':      true,
	'2':      true,
	'3':      true,
	'4':      true,
	'5':      true,
	'6':      true,
	'7':      true,
	'8':      true,
	'9':      true,
	':':      true,
	';':      true,
	'<':      true,
	'=':      true,
	'>':      true,
	'?':      true,
	'@':      true,
	'A':      true,
	'B':      true,
	'C':      true,
	'D':      true,
	'E':      true,
	'F':      true,
	'G':      true,
	'H':      true,
	'I':      true,
	'J':      true,
	'K':      true,
	'L':      true,
	'M':      true,
	'N':      true,
	'O':      true,
	'P':      true,
	'Q':      true,
	'R':      true,
	'S':      true,
	'T':      true,
	'U':      true,
	'V':      true,
	'W':      true,
	'X':      true,
	'Y':      true,
	'Z':      true,
	'[':      true,
	'\\':     false,
	']':      true,
	'^':      true,
	'_':      true,
	'`':      true,
	'a':      true,
	'b':      true,
	'c':      true,
	'd':      true,
	'e':      true,
	'f':      true,
	'g':      true,
	'h':      true,
	'i':      true,
	'j':      true,
	'k':      true,
	'l':      true,
	'm':      true,
	'n':      true,
	'o':      true,
	'p':      true,
	'q':      true,
	'r':      true,
	's':      true,
	't':      true,
	'u':      true,
	'v':      true,
	'w':      true,
	'x':      true,
	'y':      true,
	'z':      true,
	'{':      true,
	'|':      true,
	'}':      true,
	'~':      true,
	'\u007f': true,
}

// htmlSafeSet holds the value true if the ASCII character with the given
// array position can be safely represented inside a JSON string, embedded
// inside of HTML <script> tags, without any additional escaping.
//
// All values are true except for the ASCII control characters (0-31), the
// double quote ("), the backslash character ("\"), HTML opening and closing
// tags ("<" and ">"), and the ampersand ("&").
var htmlSafeSet = [utf8.RuneSelf]bool{
	' ':      true,
	'!':      true,
	'"':      false,
	'#':      true,
	'$':      true,
	'%':      true,
	'&':      false,
	'\'':     true,
	'(':      true,
	')':      true,
	'*':      true,
	'+':      true,
	',':      true,
	'-':      true,
	'.':      true,
	'/':      true,
	'0':      true,
	'1':      true,
	'2':      true,
	'3':      true,
	'4':      true,
	'5':      true,
	'6':      true,
	'7':      true,
	'8':      true,
	'9':      true,
	':':      true,
	';':      true,
	'<':      false,
	'=':      true,
	'>':      false,
	'?':      true,
	'@':      true,
	'A':      true,
	'B':      true,
	'C':      true,
	'D':      true,
	'E':      true,
	'F':      true,
	'G':      true,
	'H':      true,
	'I':      true,
	'J':      true,
	'K':      true,
	'L':      true,
	'M':      true,
	'N':      true,
	'O':      true,
	'P':      true,
	'Q':      true,
	'R':      true,
	'S':      true,
	'T':      true,
	'U':      true,
	'V':      true,
	'W':      true,
	'X':      true,
	'Y':      true,
	'Z':      true,
	'[':      true,
	'\\':     false,
	']':      true,
	'^':      true,
	'_':      true,
	'`':      true,
	'a':      true,
	'b':      true,
	'c':      true,
	'd':      true,
	'e':      true,
	'f':      true,
	'g':      true,
	'h':      true,
	'i':      true,
	'j':      true,
	'k':      true,
	'l':      true,
	'm':      true,
	'n':      true,
	'o':      true,
	'p':      true,
	'q':      true,
	'r':      true,
	's':      true,
	't':      true,
	'u':      true,
	'v':      true,
	'w':      true,
	'x':      true,
	'y':      true,
	'z':      true,
	'{':      true,
	'|':      true,
	'}':      true,
	'~':      true,
	'\u007f': true,
}

type stdLogger struct {
	sev   severity
	depth int
}

func (l *stdLogger) Write(p []byte) (n int, err error) {
	n = len(p)
	p = p[:len(p)-1]
	newStruct(l.depth, l.sev, 0).Msg(*(*string)(unsafe.Pointer(&p)))
	return
}

// ErrorLogger acts as *log.Logger but uses depth to determine which call frame to log. ErrorLogger(0).Print("msg") is the same as Errors().Msg("msg").
func ErrorLogger(depth int) *log.Logger {
	return log.New(&stdLogger{errorLog, depth + 4}, "", 0)
}
