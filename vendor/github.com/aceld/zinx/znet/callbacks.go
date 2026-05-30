package znet

// callbackNode represents a node in the callback linked list
// Each node contains handler identifier, key, callback function and pointer to next node
type callbackNode struct {
	handler any           // Handler identifier, used to identify the source or type of callback
	key     any           // Unique identifier key for callback, used in combination with handler
	call    func()        // Actual callback function to be executed
	next    *callbackNode // Pointer to next node, forming linked list structure
}

// callbacks is a singly linked list structure for managing multiple callback functions
// Supports dynamic addition, removal and execution of callbacks
type callbacks struct {
	first *callbackNode // Pointer to the first node of the linked list
	last  *callbackNode // Pointer to the last node of the linked list, used for quick addition of new nodes
}

// Add adds a new callback function to the callback linked list
// Parameters:
//   - handler: Handler identifier, can be any type
//   - key: Unique identifier key for callback, used in combination with handler
//   - callback: Callback function to be executed, ignored if nil
//
// Note: If a callback with the same handler and key already exists, it will be replaced
func (t *callbacks) Add(handler, key any, callback func()) {
	// Prevent adding empty callback function
	if callback == nil {
		return
	}

	// Check if a callback with the same handler and key already exists
	for cb := t.first; cb != nil; cb = cb.next {
		if cb.handler == handler && cb.key == key {
			// Replace existing callback
			cb.call = callback
			return
		}
	}

	// Create new callback node
	newItem := &callbackNode{handler, key, callback, nil}

	if t.first == nil {
		// If linked list is empty, new node becomes the first node
		t.first = newItem
	} else {
		// Otherwise add new node to the end of linked list
		t.last.next = newItem
	}
	// Update pointer to last node
	t.last = newItem
}

// Remove removes the specified callback function from the callback linked list
// Parameters:
//   - handler: Handler identifier of the callback to be removed
//   - key: Unique identifier key of the callback to be removed
//
// Note: If no matching callback is found, this method has no effect
func (t *callbacks) Remove(handler, key any) {
	var prev *callbackNode

	// Traverse linked list to find the node to be removed
	for callback := t.first; callback != nil; prev, callback = callback, callback.next {
		// Found matching node
		if callback.handler == handler && callback.key == key {
			if t.first == callback {
				// If it's the first node, update first pointer
				t.first = callback.next
			} else if prev != nil {
				// If it's a middle node, update the next pointer of the previous node
				prev.next = callback.next
			}

			if t.last == callback {
				// If it's the last node, update last pointer
				t.last = prev
			}

			// Return immediately after finding and removing
			return
		}
	}
}

// Invoke executes all registered callback functions in the linked list
// Executes each callback in the order they were added
// Note: If a callback function is nil, it will be skipped
// If a callback panics, it will be handled by the outer caller's panic recovery
func (t *callbacks) Invoke() {
	// Traverse the entire linked list starting from the head node
	for callback := t.first; callback != nil; callback = callback.next {
		callback.call()
	}
}

// Len returns the number of callback functions in the linked list
// Return value: Total number of currently registered callback functions
func (t *callbacks) Len() int {
	var count int

	// Traverse linked list to count
	for callback := t.first; callback != nil; callback = callback.next {
		count++
	}

	return count
}
