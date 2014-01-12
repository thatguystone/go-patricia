// Copyright (c) 2014 The go-patricia AUTHORS
//
// Use of this source code is governed by The MIT License
// that can be found in the LICENSE file.

package patricia

//import "fmt"

const (
	// Max prefix length that is kept in a single trie node.
	MaxPrefixPerNode = 10
	// Max children to keep in a node in the sparse mode.
	MaxChildrenPerSparseNode = 8
)

type childList interface {
	length() int
	head() *Trie
	add(child *Trie) childList
	replace(child *Trie)
	remove(child *Trie)
	next(b byte) *Trie
	walk(prefix *Prefix, visitor VisitorFunc) error
}

type sparseChildList struct {
	children []*Trie
}

func newSparseChildList() childList {
	return &sparseChildList{
		children: make([]*Trie, 0, MaxChildrenPerSparseNode),
	}
}

func (list *sparseChildList) length() int {
	return len(list.children)
}

func (list *sparseChildList) head() *Trie {
	return list.children[0]
}

func (list *sparseChildList) add(child *Trie) childList {
	// Search for an empty spot and insert the child if possible.
	if len(list.children) != cap(list.children) {
		list.children = append(list.children, child)
		return list
	}

	// Otherwise we have to transform to the dense list type.
	return newDenseChildList(list)
}

func (list *sparseChildList) replace(child *Trie) {
	// Seek the child with the same prefix.
	for i, node := range list.children {
		if node.prefix[0] == child.prefix[0] {
			list.children[i] = child
			return
		}
	}

	// This is not supposed to be reached.
	panic("replacing non-existent child")
}

func (list *sparseChildList) remove(child *Trie) {
	for i, node := range list.children {
		if node.prefix[0] == child.prefix[0] {
			list.children = append(list.children[:i], list.children[i+1:]...)
			return
		}
	}

	// This is not supposed to be reached.
	panic("removing non-existent child")
}

func (list *sparseChildList) next(b byte) *Trie {
	for _, child := range list.children {
		if child.prefix[0] == b {
			return child
		}
	}
	return nil
}

func (list *sparseChildList) walk(prefix *Prefix, visitor VisitorFunc) error {
	for _, child := range list.children {
		if child.item != nil {
			*prefix = append(*prefix, child.prefix...)
			err := visitor(*prefix, child.item)
			if err != nil {
				*prefix = (*prefix)[:len(*prefix)-len(child.prefix)]
				if err == SkipSubtree {
					continue
				}
				return err
			}
		}

		err := child.children.walk(prefix, visitor)
		*prefix = (*prefix)[:len(*prefix)-len(child.prefix)]
		if err != nil {
			return err
		}
	}

	return nil
}

type denseChildList struct {
	min      int
	max      int
	children []*Trie
}

func newDenseChildList(list *sparseChildList) childList {
	var (
		min int = 255
		max int = 0
	)
	for _, child := range list.children {
		b := int(child.prefix[0])
		if b < min {
			min = b
		}
		if b > max {
			max = b
		}
	}

	children := make([]*Trie, max-min+1)
	for _, child := range list.children {
		children[min+int(child.prefix[0])] = child
	}

	return &denseChildList{min, max, children}
}

func (list *denseChildList) length() int {
	return list.max - list.min
}

func (list *denseChildList) head() *Trie {
	return list.children[0]
}

func (list *denseChildList) add(child *Trie) childList {
	b := int(child.prefix[0])

	switch {
	case list.min <= b && b <= list.max:
		if list.children[b] != nil {
			panic("dense child list collision detected")
		}
		list.children[b] = child

	case b < list.min:
		children := make([]*Trie, list.max-b+1)
		children[0] = child
		copy(children[b:], list.children)
		list.children = children
		list.min = b

	default:
		children := make([]*Trie, b-list.min+1)
		children[b] = child
		copy(children, list.children)
		list.children = children
		list.max = b
	}

	return list
}

func (list *denseChildList) replace(child *Trie) {
	list.children[int(child.prefix[0])] = child
}

func (list *denseChildList) remove(child *Trie) {
	b := int(child.prefix[0])
	if list.children[b] == nil {
		// This is not supposed to be reached.
		panic("removing non-existent child")
	}
	list.children[b] = child
}

func (list *denseChildList) next(b byte) *Trie {
	i := int(b)
	if i < list.min || list.max < i {
		return nil
	}
	return list.children[i]
}

func (list *denseChildList) walk(prefix *Prefix, visitor VisitorFunc) error {
	for _, child := range list.children {
		if child == nil {
			continue
		}
		if child.item != nil {
			if err := visitor(*prefix, child.item); err != nil {
				if err == SkipSubtree {
					continue
				}
				return err
			}
		}

		*prefix = append(*prefix, child.prefix...)
		err := child.children.walk(prefix, visitor)
		*prefix = (*prefix)[:len(*prefix)-len(child.prefix)]
		if err != nil {
			return err
		}
	}

	return nil
}
