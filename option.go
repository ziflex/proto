// Copyright (c) 2017 Ernest Micklei
//
// MIT License
//
// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files (the
// "Software"), to deal in the Software without restriction, including
// without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense, and/or sell copies of the Software, and to
// permit persons to whom the Software is furnished to do so, subject to
// the following conditions:
//
// The above copyright notice and this permission notice shall be
// included in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
// LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
// OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
// WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package proto

import "fmt"
import "bytes"

// Option is a protoc compiler option
type Option struct {
	Comment             *Comment
	Name                string
	Constant            Literal
	IsEmbedded          bool
	AggregatedConstants []*NamedLiteral
	InlineComment       *Comment
}

// columns returns printable source tokens
func (o *Option) columns() (cols []aligned) {
	if !o.IsEmbedded {
		cols = append(cols, leftAligned("option "))
	} else {
		cols = append(cols, leftAligned(" ["))
	}
	cols = append(cols, o.keyValuePair(o.IsEmbedded)...)
	if o.IsEmbedded {
		cols = append(cols, leftAligned("]"))
	}
	if !o.IsEmbedded {
		cols = append(cols, alignedSemicolon)
		if o.InlineComment != nil {
			cols = append(cols, notAligned(" //"), notAligned(o.InlineComment.Message()))
		}
	}
	return
}

// keyValuePair returns key = value or "value"
func (o *Option) keyValuePair(embedded bool) (cols []aligned) {
	equals := alignedEquals
	name := o.Name
	if embedded {
		return append(cols, leftAligned(name), equals, leftAligned(o.Constant.String())) // numbers right, strings left? TODO
	}
	return append(cols, rightAligned(name), equals, rightAligned(o.Constant.String()))
}

// parse reads an Option body
// ( ident | "(" fullIdent ")" ) { "." ident } "=" constant ";"
func (o *Option) parse(p *Parser) error {
	tok, lit := p.scanIgnoreWhitespace()
	if tLEFTPAREN == tok {
		tok, lit = p.scanIgnoreWhitespace()
		if tok != tIDENT {
			if !isKeyword(tok) {
				return p.unexpected(lit, "option full identifier", o)
			}
		}
		tok, _ = p.scanIgnoreWhitespace()
		if tok != tRIGHTPAREN {
			return p.unexpected(lit, "full identifier closing )", o)
		}
		o.Name = fmt.Sprintf("(%s)", lit)
	} else {
		// non full ident
		if tIDENT != tok {
			if !isKeyword(tok) {
				return p.unexpected(lit, "option identifier", o)
			}
		}
		o.Name = lit
	}
	tok, lit = p.scanIgnoreWhitespace()
	if tDOT == tok {
		// extend identifier
		tok, lit = p.scanIgnoreWhitespace()
		if tok != tIDENT {
			return p.unexpected(lit, "option postfix identifier", o)
		}
		o.Name = fmt.Sprintf("%s.%s", o.Name, lit)
		tok, lit = p.scanIgnoreWhitespace()
	}
	if tEQUALS != tok {
		return p.unexpected(lit, "option constant =", o)
	}
	p.s.skipWhitespace()
	if p.s.peek('{') {
		p.s.read()
		return o.parseAggregate(p)
	}
	// non aggregate
	l := new(Literal)
	if err := l.parse(p); err != nil {
		return err
	}
	o.Constant = *l
	return nil
}

// inlineComment is part of commentInliner.
func (o *Option) inlineComment(c *Comment) {
	o.InlineComment = c
}

// Accept dispatches the call to the visitor.
func (o *Option) Accept(v Visitor) {
	v.VisitOption(o)
}

// Doc is part of Documented
func (o *Option) Doc() *Comment {
	return o.Comment
}

// Literal represents intLit,floatLit,strLit or boolLit
type Literal struct {
	Source   string
	IsString bool
}

// String returns the source (if quoted then use double quote).
func (l Literal) String() string {
	var buf bytes.Buffer
	if l.IsString {
		buf.WriteRune('"')
	}
	buf.WriteString(l.Source)
	if l.IsString {
		buf.WriteRune('"')
	}
	return buf.String()
}

// parse expects to read a literal constant after =.
func (l *Literal) parse(p *Parser) error {
	l.Source, l.IsString = p.s.scanLiteral()
	return nil
}

// NamedLiteral associates a name with a Literal
type NamedLiteral struct {
	*Literal
	Name string
}

// parseAggregate reads options written using aggregate syntax
func (o *Option) parseAggregate(p *Parser) error {
	o.AggregatedConstants = []*NamedLiteral{}
	for {
		tok, lit := p.scanIgnoreWhitespace()
		if tRIGHTSQUARE == tok {
			p.unscan()
			// caller has checked for open square ; will consume rightsquare, rightcurly and semicolon
			return nil
		}
		if tRIGHTCURLY == tok {
			continue
		}
		if tSEMICOLON == tok {
			return nil
		}
		if tCOMMA == tok {
			if len(o.AggregatedConstants) == 0 {
				return p.unexpected(lit, "non-empty option aggregate key", o)
			}
			continue
		}
		if tIDENT != tok {
			return p.unexpected(lit, "option aggregate key", o)
		}
		key := lit
		tok, lit = p.scanIgnoreWhitespace()
		if tCOLON != tok {
			return p.unexpected(lit, "option aggregate key colon :", o)
		}
		l := new(Literal)
		if err := l.parse(p); err != nil {
			return err
		}
		o.AggregatedConstants = append(o.AggregatedConstants, &NamedLiteral{Name: key, Literal: l})
	}
}
