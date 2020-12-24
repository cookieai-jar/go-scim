package prop

import (
	"fmt"
	"github.com/imulab/go-scim/pkg/v2/spec"
)

// Navigate returns a navigator that allows caller to freely navigate the property structure and maintains the navigation
// history to enable retraction at any time. The navigator also exposes delegate methods to modify the property, and
// propagate modification events to upstream properties.
func Navigate(property Property) Navigator {
	return &defaultNavigator{stack: []Property{property}}
}

// Navigator is a controlled mechanism to traverse the Resource/Property data structure. It should be used in cases
// where the caller has knowledge of what to access. For example, when de-serializing JSON into a Resource, caller
// has knowledge of the JSON structure, therefore knows what to access in the Resource structure.
//
// The Navigator exists for two purposes: First, to maintain a call stack of the traversal trace, enabling modification
// events to be propagated along the trace, all the way to the root property, and notify subscribers on every level.
// Without such call stack, such things will have to resort to runtime reflection, which is hard to get right or maintain.
// Second, as a frequently used feature, the Navigator promotes fluent API by returning the instance for the majority of
// the accessor methods, making the code easier to read in general.
//
// Because the Navigator does not return error on every error-able operation, implementation will mostly be stateful. Callers
// should call Error or HasError to check if the Navigator is currently in the error state, after performing one or possibly
// several chained operations fluently.
//
// The call stack can be advanced by calling Dot, At and Where methods. Which methods to call depends on the context. The
// call stack can be retracted by calling Retract. The top and the bottom item on the stack can be queried by calling
// Current and Source. The stack depth is available via Depth.
type Navigator interface {
	// Error returns any error occurred during fluent navigation. If any step
	// during the navigation had generated an error, further steps will become
	// no op and the original error is reflected here.
	Error() error
	// HasError returns true when Error is not nil.
	HasError() bool
	// ClearError resets the error in the navigator.
	ClearError()
	// Depth return the number of properties that was focused, including the
	// currently focused. These properties, excluding the current one, can be
	// refocused by calling Retract, one at a time in the reversed order that
	// were focused. The minimum depth is one.
	Depth() int
	// Source returns the property this Navigator is created with.
	Source() Property
	// Current returns the currently focused property on the top of the trace stack.
	Current() Property
	// Retract goes back to the last focused property. The source property that
	// this navigator was created with cannot be retracted
	Retract() Navigator
	// Dot focuses on the sub property that goes by the given name (case insensitive)
	Dot(name string) Navigator
	// At focuses on the element property at given index
	At(index int) Navigator
	// Where focuses on the first child property meeting given criteria
	Where(criteria func(child Property) bool) Navigator
	// Add delegates for Add of the Current property and propagates events to upstream properties.
	Add(value interface{}) Navigator
	// Replace delegates for Replace of the Current property and propagates events to upstream properties.
	Replace(value interface{}) Navigator
	// Delete delegates for Delete of the Current property and propagates events to upstream properties.
	Delete() Navigator
	// ForEachChild iterates each child property of the current property and invokes callback.
	// The method returns any error generated previously or generated by any of the callbacks.
	ForEachChild(callback func(index int, child Property) error) error
}

type defaultNavigator struct {
	stack []Property
	err   error
}

func (n *defaultNavigator) Error() error {
	return n.err
}

func (n *defaultNavigator) HasError() bool {
	return n.err != nil
}

func (n *defaultNavigator) ClearError() {
	n.err = nil
}

func (n *defaultNavigator) Depth() int {
	return len(n.stack)
}

func (n *defaultNavigator) Source() Property {
	return n.stack[0]
}

func (n *defaultNavigator) Current() Property {
	return n.stack[len(n.stack)-1]
}

func (n *defaultNavigator) Retract() Navigator {
	if n.Depth() > 1 {
		n.stack = n.stack[:len(n.stack)-1]
	}
	return n
}

func (n *defaultNavigator) Dot(name string) Navigator {
	if n.err != nil {
		return n
	}

	child, err := n.Current().ChildAtIndex(name)
	if err != nil {
		n.err = fmt.Errorf("%w: no attribute named '%s' from '%s'", spec.ErrInvalidPath, name, n.Current().Attribute().Path())
		return n
	}

	n.stack = append(n.stack, child)
	return n
}

func (n *defaultNavigator) At(index int) Navigator {
	if n.err != nil {
		return n
	}

	child, err := n.Current().ChildAtIndex(index)
	if err != nil {
		n.err = fmt.Errorf("%w: no target at index '%d' from '%s'", spec.ErrNoTarget, index, n.Current().Attribute().Path())
		return n
	}

	n.stack = append(n.stack, child)
	return n
}

func (n *defaultNavigator) Where(criteria func(child Property) bool) Navigator {
	if n.err != nil {
		return n
	}

	child := n.Current().FindChild(criteria)
	if child == nil {
		n.err = fmt.Errorf("%w: no target meeting criteria from '%s'", spec.ErrNoTarget, n.Current().Attribute().Path())
		return n
	}

	n.stack = append(n.stack, child)
	return n
}

func (n *defaultNavigator) ForEachChild(callback func(index int, child Property) error) error {
	if n.err != nil {
		return n.err
	}
	return n.Current().ForEachChild(callback)
}

// Add delegates for Add of the Current property and propagates events to upstream properties.
func (n *defaultNavigator) Add(value interface{}) Navigator {
	n.err = n.delegateMod(func() (event *Event, err error) {
		return n.Current().Add(value)
	})
	return n
}

// Replace delegates for Replace of the Current property and propagates events to upstream properties.
func (n *defaultNavigator) Replace(value interface{}) Navigator {
	n.err = n.delegateMod(func() (event *Event, err error) {
		return n.Current().Replace(value)
	})
	return n
}

// Delete delegates for Delete of the Current property and propagates events to upstream properties.
func (n *defaultNavigator) Delete() Navigator {
	n.err = n.delegateMod(func() (event *Event, err error) {
		return n.Current().Delete()
	})
	return n
}

func (n *defaultNavigator) delegateMod(mod func() (*Event, error)) error {
	if n.err != nil {
		return n.err
	}

	ev, err := mod()
	if err != nil {
		return err
	}

	if ev != nil {
		events := ev.ToEvents()
		for i := len(n.stack) - 1; i >= 0; i-- {
			if err := n.stack[i].Notify(events); err != nil {
				return err
			}
		}
	}

	return nil
}
