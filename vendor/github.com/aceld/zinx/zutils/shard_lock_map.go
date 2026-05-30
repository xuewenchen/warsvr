// Package zutils provides utility functions and data structures for the Zinx framework.
// This package includes a high-performance sharded concurrent map implementation.
package zutils

import (
	"encoding/json"
	"sync"
)

// DefaultShardCount is the default number of shards for the concurrent map.
// A higher number reduces lock contention but increases memory overhead.
var DefaultShardCount = 32

// ShardLockMaps is a thread-safe map of type string:Anything.
// To avoid lock bottlenecks, this map is divided into several shards.
// Each shard has its own read-write mutex, allowing concurrent access
// to different shards without blocking.
type ShardLockMaps struct {
	shards     []*SingleShardMap
	hash       IHash
	shardCount int
}

// SingleShardMap is a thread-safe string to anything map.
// It represents a single shard within the ShardLockMaps.
type SingleShardMap struct {
	items map[string]interface{}
	sync.RWMutex
}

// createShardLockMaps Creates a new concurrent map.
func createShardLockMaps(hash IHash, shardCount int) ShardLockMaps {
	slm := ShardLockMaps{
		shards:     make([]*SingleShardMap, shardCount),
		hash:       hash,
		shardCount: shardCount,
	}
	for i := 0; i < shardCount; i++ {
		slm.shards[i] = &SingleShardMap{items: make(map[string]interface{})}
	}
	return slm
}

// NewShardLockMaps creates a new ShardLockMaps with default shard count.
// Example usage:
//
//	m := NewShardLockMaps()
//	m.Set("key", "value")
//	if val, ok := m.Get("key"); ok {
//	    fmt.Println(val)
//	}
func NewShardLockMaps() ShardLockMaps {
	return createShardLockMaps(DefaultHash(), DefaultShardCount)
}

// NewShardLockMapsWithCount creates a new ShardLockMaps with custom shard count.
// Use this when you need to tune performance based on your workload.
// More shards = less lock contention but more memory overhead.
func NewShardLockMapsWithCount(shardCount int) ShardLockMaps {
	return createShardLockMaps(DefaultHash(), shardCount)
}

// NewWithCustomHash creates a new ShardLockMaps with custom hash function.
// Use this when you need a different hash distribution strategy.
func NewWithCustomHash(hash IHash) ShardLockMaps {
	return createShardLockMaps(hash, DefaultShardCount)
}

// NewWithCustomHashAndCount creates a new ShardLockMaps with custom hash function and shard count.
// This provides maximum flexibility for performance tuning.
func NewWithCustomHashAndCount(hash IHash, shardCount int) ShardLockMaps {
	return createShardLockMaps(hash, shardCount)
}

// GetShard returns shard under given key
func (slm ShardLockMaps) GetShard(key string) *SingleShardMap {
	return slm.shards[slm.hash.Sum(key)%uint32(slm.shardCount)]
}

// Count returns the number of elements within the map.
func (slm ShardLockMaps) Count() int {
	count := 0
	for i := 0; i < slm.shardCount; i++ {
		shard := slm.shards[i]
		shard.RLock()
		count += len(shard.items)
		shard.RUnlock()
	}
	return count
}

// Get retrieves an element from map under given key.
func (slm ShardLockMaps) Get(key string) (interface{}, bool) {
	shard := slm.GetShard(key)
	shard.RLock()
	val, ok := shard.items[key]
	shard.RUnlock()
	return val, ok
}

// Set Sets the given value under the specified key.
func (slm ShardLockMaps) Set(key string, value interface{}) {
	shard := slm.GetShard(key)
	shard.Lock()
	shard.items[key] = value
	shard.Unlock()
}

// SetNX Sets the given value under the specified key if no value was associated with it.
func (slm ShardLockMaps) SetNX(key string, value interface{}) bool {
	shard := slm.GetShard(key)
	shard.Lock()
	_, ok := shard.items[key]
	if !ok {
		shard.items[key] = value
	}
	shard.Unlock()
	return !ok
}

// MSet Sets the given value under the specified key.
func (slm ShardLockMaps) MSet(data map[string]interface{}) {
	for key, value := range data {
		shard := slm.GetShard(key)
		shard.Lock()
		shard.items[key] = value
		shard.Unlock()
	}
}

// Has Looks up an item under specified key
func (slm ShardLockMaps) Has(key string) bool {
	shard := slm.GetShard(key)
	shard.RLock()
	_, ok := shard.items[key]
	shard.RUnlock()
	return ok
}

// Remove removes an element from the map.
func (slm ShardLockMaps) Remove(key string) {
	shard := slm.GetShard(key)
	shard.Lock()
	delete(shard.items, key)
	shard.Unlock()
}

// RemoveCb is a callback executed in a map.RemoveCb() call, while Lock is held
// If returns true, the element will be removed from the map
type RemoveCb func(key string, v interface{}, exists bool) bool

// RemoveCb locks the shard containing the key, retrieves its current value and calls the callback with those params
// If callback returns true and element exists, it will remove it from the map
// Returns the value returned by the callback (even if element was not present in the map)
func (slm ShardLockMaps) RemoveCb(key string, cb RemoveCb) bool {

	shard := slm.GetShard(key)
	shard.Lock()
	v, ok := shard.items[key]
	remove := cb(key, v, ok)
	if remove && ok {
		delete(shard.items, key)
	}
	shard.Unlock()
	return remove
}

// Pop removes an element from the map and returns it
func (slm ShardLockMaps) Pop(key string) (v interface{}, exists bool) {
	shard := slm.GetShard(key)
	shard.Lock()
	v, exists = shard.items[key]
	delete(shard.items, key)
	shard.Unlock()
	return v, exists
}

// GetOrSet gets the value for the given key, or sets it if it doesn't exist.
// Returns the value and whether it was set (true) or already existed (false).
func (slm ShardLockMaps) GetOrSet(key string, value interface{}) (interface{}, bool) {
	if v, ok := slm.Get(key); ok {
		return v, false
	}
	return slm.doSetWithLockCheck(key, value)
}

// GetOrSetFunc gets the value for the given key, or sets it using the provided function if it doesn't exist.
// The function f is called outside the lock to avoid deadlocks.
// Returns the value and whether it was set (true) or already existed (false).
func (slm ShardLockMaps) GetOrSetFunc(key string, f func(key string) interface{}) (interface{}, bool) {
	if v, ok := slm.Get(key); ok {
		return v, false
	}
	return slm.doSetWithLockCheck(key, f(key))
}

// GetOrSetFuncLock gets the value for the given key, or sets it using the provided function if it doesn't exist.
// The function f is called inside the lock for atomic operations.
// WARNING: Do not perform operations on the container within f to avoid deadlocks.
// Returns the value and whether it was set (true) or already existed (false).
func (slm ShardLockMaps) GetOrSetFuncLock(key string, f func(key string) interface{}) (interface{}, bool) {
	if v, ok := slm.Get(key); ok {
		return v, false
	}
	return slm.doSetWithLockCheckWithFunc(key, f)
}

// doSetWithLockCheck performs a set operation with lock checking
func (slm ShardLockMaps) doSetWithLockCheck(key string, val interface{}) (interface{}, bool) {
	shard := slm.GetShard(key)
	shard.Lock()
	defer shard.Unlock()

	if got, ok := shard.items[key]; ok {
		return got, false
	}

	shard.items[key] = val
	return val, true
}

// doSetWithLockCheckWithFunc performs a set operation with function execution inside lock
func (slm ShardLockMaps) doSetWithLockCheckWithFunc(key string, f func(key string) interface{}) (interface{}, bool) {
	shard := slm.GetShard(key)
	shard.Lock()
	defer shard.Unlock()

	if got, ok := shard.items[key]; ok {
		return got, false
	}

	val := f(key)
	shard.items[key] = val
	return val, true
}

// Clear removes all items from map.
func (slm ShardLockMaps) Clear() {
	for item := range slm.IterBuffered() {
		slm.Remove(item.Key)
	}
}

// LockFuncWithKey executes a function with write lock on the shard containing the key.
// WARNING: Do not perform operations on the container within f to avoid deadlocks.
func (slm ShardLockMaps) LockFuncWithKey(key string, f func(shardData map[string]interface{})) {
	shard := slm.GetShard(key)
	shard.Lock()
	defer shard.Unlock()
	f(shard.items)
}

// RLockFuncWithKey executes a function with read lock on the shard containing the key.
// WARNING: Do not perform write operations on the container within f to avoid deadlocks.
func (slm ShardLockMaps) RLockFuncWithKey(key string, f func(shardData map[string]interface{})) {
	shard := slm.GetShard(key)
	shard.RLock()
	defer shard.RUnlock()
	f(shard.items)
}

// LockFunc executes a function with write lock on all shards.
// WARNING: Do not perform operations on the container within f to avoid deadlocks.
func (slm ShardLockMaps) LockFunc(f func(shardData map[string]interface{})) {
	for _, shard := range slm.shards {
		shard.Lock()
		f(shard.items)
		shard.Unlock()
	}
}

// RLockFunc executes a function with read lock on all shards.
// WARNING: Do not perform write operations on the container within f to avoid deadlocks.
func (slm ShardLockMaps) RLockFunc(f func(shardData map[string]interface{})) {
	for _, shard := range slm.shards {
		shard.RLock()
		f(shard.items)
		shard.RUnlock()
	}
}

// ClearWithFuncLock clears all items with a callback function executed under lock.
// WARNING: Do not perform operations on the container within onClear to avoid deadlocks.
func (slm ShardLockMaps) ClearWithFuncLock(onClear func(key string, val interface{})) {
	for _, shard := range slm.shards {
		shard.Lock()
		for key, val := range shard.items {
			onClear(key, val)
		}
		shard.items = make(map[string]interface{})
		shard.Unlock()
	}
}

// IsEmpty checks if map is empty.
func (slm ShardLockMaps) IsEmpty() bool {
	return slm.Count() == 0
}

// MGet retrieves multiple elements from the map.
func (slm ShardLockMaps) MGet(keys ...string) map[string]interface{} {
	data := make(map[string]interface{})
	for _, key := range keys {
		if val, ok := slm.Get(key); ok {
			data[key] = val
		}
	}
	return data
}

// GetAll returns a copy of all items in the map.
func (slm ShardLockMaps) GetAll() map[string]interface{} {
	data := make(map[string]interface{})
	for _, shard := range slm.shards {
		shard.RLock()
		for key, val := range shard.items {
			data[key] = val
		}
		shard.RUnlock()
	}
	return data
}

// Tuple Used by the IterBuffered functions to wrap two variables together over a channel,
type Tuple struct {
	Key string
	Val interface{}
}

// Returns a array of channels that contains elements in each shard,
// which likely takes a snapshot of `slm`.
// It returns once the size of each buffered channel is determined,
// before all the channels are populated using goroutines.
func snapshot(slm ShardLockMaps) (chanList []chan Tuple) {
	chanList = make([]chan Tuple, slm.shardCount)
	wg := sync.WaitGroup{}
	wg.Add(slm.shardCount)
	for index, shard := range slm.shards {
		go func(index int, shard *SingleShardMap) {
			shard.RLock()
			chanList[index] = make(chan Tuple, len(shard.items))
			wg.Done()
			for key, val := range shard.items {
				chanList[index] <- Tuple{key, val}
			}
			shard.RUnlock()
			close(chanList[index])
		}(index, shard)
	}
	wg.Wait()
	return chanList
}

// fanIn reads elements from channels `chanList` into channel `out`
func fanIn(chanList []chan Tuple, out chan Tuple) {
	wg := sync.WaitGroup{}
	wg.Add(len(chanList))
	for _, ch := range chanList {
		go func(ch chan Tuple) {
			for t := range ch {
				out <- t
			}
			wg.Done()
		}(ch)
	}
	wg.Wait()
	close(out)
}

// IterBuffered returns a buffered iterator which could be used in a for range loop.
func (slm ShardLockMaps) IterBuffered() <-chan Tuple {
	chanList := snapshot(slm)
	total := 0
	for _, c := range chanList {
		total += cap(c)
	}
	ch := make(chan Tuple, total)
	go fanIn(chanList, ch)
	return ch
}

// Items returns all items as map[string]interface{}
func (slm ShardLockMaps) Items() map[string]interface{} {
	tmp := make(map[string]interface{})

	for item := range slm.IterBuffered() {
		tmp[item.Key] = item.Val
	}

	return tmp
}

// Keys returns all keys as []string
func (slm ShardLockMaps) Keys() []string {
	count := slm.Count()
	ch := make(chan string, count)
	go func() {
		wg := sync.WaitGroup{}
		wg.Add(slm.shardCount)
		for _, shard := range slm.shards {
			go func(shard *SingleShardMap) {
				shard.RLock()
				for key := range shard.items {
					ch <- key
				}
				shard.RUnlock()
				wg.Done()
			}(shard)
		}
		wg.Wait()
		close(ch)
	}()

	keys := make([]string, 0, count)
	for k := range ch {
		keys = append(keys, k)
	}
	return keys
}

// IterCb Iterator callback,called for every key,value found in maps.
// RLock is held for all calls for a given shard
// therefore callback sess consistent view of a shard,
// but not across the shards
type IterCb func(key string, v interface{})

// IterCb Callback based iterator, cheapest way to read
// all elements in a map.
func (slm ShardLockMaps) IterCb(fn IterCb) {
	for idx := range slm.shards {
		shard := (slm.shards)[idx]
		shard.RLock()
		for key, value := range shard.items {
			fn(key, value)
		}
		shard.RUnlock()
	}
}

// MarshalJSON Reviles ConcurrentMap "private" variables to json marshal.
func (slm ShardLockMaps) MarshalJSON() ([]byte, error) {
	tmp := make(map[string]interface{})

	for item := range slm.IterBuffered() {
		tmp[item.Key] = item.Val
	}
	return json.Marshal(tmp)
}

// UnmarshalJSON Reverse process of Marshal.
func (slm ShardLockMaps) UnmarshalJSON(b []byte) (err error) {
	tmp := make(map[string]interface{})

	if err := json.Unmarshal(b, &tmp); err != nil {
		return err
	}

	for key, val := range tmp {
		slm.Set(key, val)
	}
	return nil
}
