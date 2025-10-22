/*
Package purefunccore provides functional bindings for Go's standard library interfaces.

# Overview

Purefunccore eliminates the need for mock generation in tests by replacing
interface-based designs with functional types. Each functional type implements
its corresponding standard library interface while providing additional methods
for composition, transformation, and testing.

# Key Benefits

  - Zero mock generation: Use inline functions instead of creating mock structs
  - Backwards compatible: Add methods without breaking existing code
  - Rich composition: Monoid operations, map, filter, and more
  - Better testing: Swap implementations without complex mock frameworks
  - Simpler code: Less boilerplate, more clarity

# Quick Example

Instead of creating interfaces and mocks:

	type Database interface {
	    Query(string) ([]Row, error)
	}

	// Need mock struct and framework...

Use functional types directly:

	type QueryFunc func(string) ([]Row, error)

	// Production
	query := QueryFunc(func(sql string) ([]Row, error) {
	    return db.Query(sql)
	})

	// Testing - NO MOCKS!
	query := QueryFunc(func(sql string) ([]Row, error) {
	    return []Row{{ID: 1}}, nil
	})

# Core Concepts

Monoids: Every type provides Empty() and Compose() operations:

	reader1.Compose(reader2) // Sequential composition
	writer1.Compose(writer2) // Tee/broadcast

Functors: Transform data with Map:

	reader.Map(bytes.ToUpper)  // Transform as you read
	writer.Map(addTimestamp)   // Transform before writing

Combinators: Rich set of operations:

	reader.Filter(predicate).Take(100).Retry(3)
	handler.WithAuth(auth).WithLogging(log).WithTimeout(5*time.Second)

# Available Types

IO Package:
  - ReadFunc: Functional io.Reader with Map, Filter, Take, Retry, etc.
  - WriteFunc: Functional io.Writer with Tee, Map, Filter, etc.
  - CloseFunc, SeekFunc, ReadAtFunc, WriteAtFunc

HTTP Package:
  - HandlerFunc: http.Handler with middleware composition
  - RoundTripperFunc: http.RoundTripper for custom HTTP clients

Context Package:
  - ContextFunc: Custom context implementations

Error Handling:
  - ErrorFunc: Composable errors with Wrap and WithCode

Formatting:
  - StringerFunc: fmt.Stringer with monoid operations

Encoding:
  - MarshalerFunc, UnmarshalerFunc: JSON marshaling
  - TextMarshalerFunc, TextUnmarshalerFunc: Text encoding

Filesystem:
  - FSFunc, FileFunc, FileInfoFunc, DirEntryFunc

Database:
  - DriverFunc, ConnFunc, StmtFunc

Sorting:
  - SortInterface: Functional sort.Interface

# Testing Philosophy

Traditional approach requires mocks:

	// Need interface
	type EmailSender interface {
	    Send(to, subject, body string) error
	}

	// Need mock struct
	type mockEmailSender struct { ... }

	// Need mock setup
	mock := &mockEmailSender{...}

Purefunccore approach - just functions:

	type SendEmailFunc func(to, subject, body string) error

	// Production
	send := SendEmailFunc(smtpClient.Send)

	// Testing - just swap!
	var captured []string
	send := SendEmailFunc(func(to, subject, body string) error {
	    captured = append(captured, to)
	    return nil
	})

No mock generation, no mock verification, just simple function substitution.

# Package Import

	import pfc "github.com/Pure-Company/purefunccore"

	// Or full import
	import "github.com/Pure-Company/purefunccore"
*/
package purefunccore
