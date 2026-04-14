package store

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// MemoryStore is an in-memory Store implementation for local development.
type MemoryStore struct {
	mu       sync.RWMutex
	nextUID  int64
	nextPID  int64
	users    map[string]*User
	posts    map[int64]*Post
	postList []int64
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nextUID: 1,
		nextPID: 1,
		users:   make(map[string]*User),
		posts:   make(map[int64]*Post),
	}
}

func (m *MemoryStore) CreateUser(user *User) error {
	if user == nil || user.DID == "" {
		return fmt.Errorf("invalid user")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.users[user.DID]; ok {
		user.ID = existing.ID
		user.CreatedAt = existing.CreatedAt
		return nil
	}

	copyUser := *user
	copyUser.ID = m.nextUID
	m.nextUID++
	if copyUser.CreatedAt.IsZero() {
		copyUser.CreatedAt = time.Now()
	}
	m.users[user.DID] = &copyUser
	user.ID = copyUser.ID
	user.CreatedAt = copyUser.CreatedAt
	return nil
}

func (m *MemoryStore) GetUserByDID(did string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, ok := m.users[did]
	if !ok {
		return nil, nil
	}
	copyUser := *u
	return &copyUser, nil
}

func (m *MemoryStore) CreatePost(post *Post) error {
	if post == nil {
		return fmt.Errorf("post is nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	copyPost := *post
	copyPost.ID = m.nextPID
	m.nextPID++
	if copyPost.CreatedAt.IsZero() {
		copyPost.CreatedAt = time.Now()
	}

	m.posts[copyPost.ID] = &copyPost
	m.postList = append(m.postList, copyPost.ID)

	post.ID = copyPost.ID
	post.CreatedAt = copyPost.CreatedAt
	return nil
}

func (m *MemoryStore) GetPosts(limit, offset int) ([]Post, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]int64, 0, len(m.postList))
	ids = append(ids, m.postList...)
	sort.Slice(ids, func(i, j int) bool {
		a := m.posts[ids[i]]
		b := m.posts[ids[j]]
		if a.VisibilityScore == b.VisibilityScore {
			return a.CreatedAt.After(b.CreatedAt)
		}
		return a.VisibilityScore > b.VisibilityScore
	})

	if offset >= len(ids) {
		return []Post{}, nil
	}
	end := offset + limit
	if end > len(ids) {
		end = len(ids)
	}

	res := make([]Post, 0, end-offset)
	for _, id := range ids[offset:end] {
		p := m.posts[id]
		res = append(res, *p)
	}
	return res, nil
}

func (m *MemoryStore) UpdateVisibilityScore(postID int64, score float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.posts[postID]
	if !ok {
		return fmt.Errorf("post not found")
	}
	p.VisibilityScore += score
	return nil
}

func (m *MemoryStore) CheckRateLimit(_ string, _ TrustTier) (bool, error) {
	return true, nil
}
