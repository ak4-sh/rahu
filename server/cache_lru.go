package server

import (
	"container/list"

	"rahu/lsp"
)

const defaultMaxCachedModules = 256

type snapshotLRUEntry struct {
	uri  lsp.DocumentURI
	name string
}

type snapshotLRU struct {
	order *list.List
	byURI map[lsp.DocumentURI]*list.Element
}

func newSnapshotLRU() *snapshotLRU {
	return &snapshotLRU{
		order: list.New(),
		byURI: make(map[lsp.DocumentURI]*list.Element),
	}
}

func (lru *snapshotLRU) touch(uri lsp.DocumentURI, name string) {
	if lru == nil || uri == "" {
		return
	}
	if elem, ok := lru.byURI[uri]; ok {
		entry := elem.Value.(*snapshotLRUEntry)
		entry.name = name
		lru.order.MoveToBack(elem)
		return
	}
	elem := lru.order.PushBack(&snapshotLRUEntry{uri: uri, name: name})
	lru.byURI[uri] = elem
}

func (lru *snapshotLRU) remove(uri lsp.DocumentURI) {
	if lru == nil || uri == "" {
		return
	}
	elem, ok := lru.byURI[uri]
	if !ok {
		return
	}
	lru.order.Remove(elem)
	delete(lru.byURI, uri)
}

func (lru *snapshotLRU) oldest() (snapshotLRUEntry, bool) {
	if lru == nil {
		return snapshotLRUEntry{}, false
	}
	front := lru.order.Front()
	if front == nil {
		return snapshotLRUEntry{}, false
	}
	return *front.Value.(*snapshotLRUEntry), true
}

func (lru *snapshotLRU) len() int {
	if lru == nil {
		return 0
	}
	return len(lru.byURI)
}
