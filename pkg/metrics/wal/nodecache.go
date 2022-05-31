package wal

//
//import (
//	"container/list"
//	"fmt"
//	"sort"
//	"sync"
//	"time"
//)
//
//type NodeCache struct {
//	stringMut sync.Mutex
//	// sort this and make it a binary search
//	entryCache []*entrylabel
//
//	// Entries should be by name and each have a mutex
//	entriesMut sync.Mutex
//	entries    map[string]*nameEntry
//	refidMap   map[uint64]*entry
//
//	refmut sync.Mutex
//	refid  uint64
//}
//
//type nameEntry struct {
//	entryMut sync.Mutex
//	name     string
//	entry    *entry
//}
//
//type entry struct {
//	refid        uint64
//	label        *entrylabel
//	children     *list.List
//	handleCount  int
//	lastAccessed time.Time
//}
//
//type entrylabel struct {
//	name, value string
//}
//
//type Label struct {
//	Name  string
//	Value string
//}
//
//func NewTree() *NodeCache {
//	return &NodeCache{
//		entryCache: make([]*entrylabel, 0),
//		entries:    make(map[string]*nameEntry),
//		refmut:     sync.Mutex{},
//		refid:      0,
//	}
//}
//
//func (t *NodeCache) AddLabelSet(labels []Label) uint64 {
//	sorted := t.sortLabels(labels)
//	refId := t.findEntry(t.entries, sorted)
//	if refId != 0 {
//		return refId
//	}
//	closest, closestLabels := t.findClosestEntry(t.entries, sorted)
//	for _, cl := range closestLabels {
//		closest = t.createEntry(closest, cl)
//	}
//	return closest.refid
//}
//
//func (t *NodeCache) GetByRefId(refId uint64) ([]Label, error) {
//
//	entry, found := t.refidMap[refId]
//	if !found {
//		return nil, fmt.Errorf("unable to find entry for refid %d", refId)
//	}
//	return entry.refid, nil
//}
//
//func (t *NodeCache) GetNameEntry(labels []Label) *nameEntry {
//	name := ""
//	for _, l := range labels {
//		if l.Name == "__name___" {
//			name = l.Name
//			break
//		}
//	}
//	if name != "" {
//
//	}
//
//}
//
//func (t *NodeCache) findClosestEntry(entries *list.List, labels []Label) (*entry, []Label) {
//	ele := entries.Front()
//	for ele != nil {
//		e := ele.Value.(*entry)
//		if *e.labelName == labels[0].Name && *e.labelValue == labels[0].Value {
//			next, _ := t.findClosestEntry(e.children, labels[1:])
//			if next == nil {
//				return e, labels
//			}
//		}
//		ele = ele.Next()
//	}
//	return nil, nil
//}
//
//func (t *NodeCache) findEntry(entries *list.List, labels []Label) uint64 {
//	ele := entries.Front()
//	for ele != nil {
//		e := ele.Value.(*entry)
//		if *e.labelName == labels[0].Name && *e.labelValue == labels[0].Value {
//			// if this is the final label we can return
//			if len(labels) == 1 {
//				return e.refid
//			}
//			t.findEntry(e.children, labels[1:])
//		}
//		ele = ele.Next()
//	}
//	return 0
//}
//
//func (t *NodeCache) createEntry(parent *entry, l Label) *entry {
//	t.entryMut.Lock()
//	defer t.entryMut.Unlock()
//	ne := &entry{
//		refid:      t.newRefID(),
//		labelName:  t.getString(l.Name),
//		labelValue: t.getString(l.Value),
//		children:   nil,
//	}
//	if parent != nil {
//		if parent.children == nil {
//			parent.children = list.New()
//		}
//		ele := parent.children.Front()
//		if ele == nil {
//			parent.children.PushFront(ne)
//			return ne
//		}
//		eleName := l.Name + l.Value
//		for ele != nil {
//			previous := ele
//			prevEntry := previous.Value.(*entry)
//			ele = ele.Next()
//			// this element belongs in the back
//			if ele == nil {
//				parent.children.PushBack(ne)
//				break
//			}
//			current := ele.Value.(*entry)
//			currentName := *current.labelName + *current.labelValue
//			previousName := *prevEntry.labelName + *prevEntry.labelValue
//
//			if eleName > currentName && eleName < previousName {
//				parent.children.InsertAfter(ne, previous)
//			}
//
//		}
//	}
//	t.refidMap[ne.refid] = ne
//	return ne
//}
//
//func (t *NodeCache) getString(input string) *string {
//	t.stringMut.Lock()
//	defer t.stringMut.Unlock()
//	for _, s := range t.entryCache {
//		if *s == input {
//			return s
//		}
//	}
//	newString := input
//	t.entryCache = append(t.entryCache, &newString)
//	return &newString
//}
//
//func (t *NodeCache) sortLabels(labels []Label) []Label {
//	keys := make([]string, len(labels))
//	list := make(map[string]Label, 0)
//	for i, k := range labels {
//		keys[i] = k.Name
//		list[k.Name] = k
//	}
//	sort.Strings(keys)
//	sorted := make([]Label, len(labels))
//	for i, l := range keys {
//		sorted[i] = list[l]
//	}
//	return sorted
//}
//
//func (t *NodeCache) newRefID() uint64 {
//	t.refmut.Lock()
//	defer t.refmut.Unlock()
//	t.refid = t.refid + 1
//	return t.refid
//}
