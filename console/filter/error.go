package filter

import "fmt"

// HumanError returns a more human-readable error for errList. It only outputs the first one.
func HumanError(err error) string {
	el, ok := err.(errList)
	if !ok {
		return err.Error()
	}
	if len(el) == 0 {
		return ""
	}
	switch e := el[0].(type) {
	case *parserError:
		return fmt.Sprintf("at line %d, position %d: %s", e.pos.line, e.pos.col, e.Inner.Error())
	default:
		return e.Error()
	}
}
