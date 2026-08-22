package person

import (
	"sort"
	"sync"
	"time"

	"gonest.dev/gonest"

	"full-text-search/shared/entity"
	"full-text-search/shared/search"
)

type Service struct {
	mu    sync.Mutex
	store []*entity.Person
}

func (s *Service) List() []*entity.Person {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*entity.Person{}, s.store...)
}

func (s *Service) Get(id string) *entity.Person {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.store {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func (s *Service) Create(props BodyCreateDTO) *entity.Person {
	p := entity.NewPerson(entity.PersonProps(props))
	s.mu.Lock()
	s.store = append(s.store, p)
	s.mu.Unlock()
	return p
}

func (s *Service) Update(id string, props BodyUpdateDTO) *entity.Person {
	p := s.Get(id)
	if p == nil {
		return nil
	}
	props.Name.Sync(&p.PersonProps.Name)
	props.Age.Sync(&p.PersonProps.Age)
	props.IsActive.Sync(&p.PersonProps.IsActive)
	props.Picture.Sync(&p.PersonProps.Picture)
	p.UpdatedAt = time.Now()
	return p
}

func (s *Service) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, p := range s.store {
		if p.ID == id {
			now := time.Now()
			p.DeletedAt = &now
			s.store = append(s.store[:i], s.store[i+1:]...)
			return true
		}
	}
	return false
}

// Search applies q.Text/Where/Sort/Pagination against the in-memory store,
// returning a search.Result the same shape any real (DB-backed) Search
// implementation of this generic query API would. Only the Where predicate
// below is Person-specific -- every operator comparison it delegates to
// lives on the search.Match* types themselves (shared/search/search.go).
func (s *Service) Search(q QueryDTO) *search.Result[*entity.Person] {
	items := s.List()

	if q.Text.IsDirty() {
		if v := q.Text.Get(); v != nil && *v != "" {
			filtered := items[:0]
			for _, p := range items {
				if search.LikeMatch(p.Name.Get(), *v) {
					filtered = append(filtered, p)
				}
			}
			items = filtered
		}
	}

	if q.Where != nil {
		filtered := make([]*entity.Person, 0, len(items))
		for _, p := range items {
			if matchWhere(p, q.Where) {
				filtered = append(filtered, p)
			}
		}
		items = filtered
	}

	applySort(items, q.Sort)

	total := int64(len(items))
	offset, limit := paginationBounds(q.Offset, q.Limit, total)
	items = items[offset:limit]

	return &search.Result[*entity.Person]{
		Items:  items,
		Total:  total,
		Offset: offset,
		Limit:  limit - offset,
	}
}

// matchWhere is the one Person-specific piece of Search: it maps each
// Where field to the Person value it filters. search.MatchField does the
// actual work (IsDirty check + Match dispatch) generically -- this function
// is now just the field-by-field wiring between Where and entity.Person, no
// per-type branching left to hand-write.
func matchWhere(p *entity.Person, w *QueryDTOWhere) bool {
	name := p.Name.Get()
	age := p.Age.Get()
	isActive := p.IsActive.Get()
	return search.MatchField(w.ID, &p.ID) &&
		search.MatchField(w.Name, &name) &&
		search.MatchField(w.Age, &age) &&
		search.MatchField(w.IsActive, &isActive) &&
		search.MatchField(w.Picture, p.Picture.Get()) &&
		search.MatchField(w.CreatedAt, &p.CreatedAt) &&
		search.MatchField(w.UpdatedAt, &p.UpdatedAt) &&
		search.MatchField(w.DeletedAt, p.DeletedAt)
}

func applySort(items []*entity.Person, fields []search.SortField[entity.Person]) {
	// Stable multi-key sort: apply LAST field first, then each earlier field
	// on top of it (sort.SliceStable's own stability guarantee makes each
	// pass preserve the previous pass's relative order for equal keys).
	for i := len(fields) - 1; i >= 0; i-- {
		field := fields[i]
		sort.SliceStable(items, func(a, b int) bool {
			less := sortLess(items[a], items[b], field.Field)
			if field.Order == search.SortDirectionDESC {
				return !less && sortLess(items[b], items[a], field.Field)
			}
			return less
		})
	}
}

func sortLess(a, b *entity.Person, field string) bool {
	switch field {
	case "id":
		return a.ID < b.ID
	case "name":
		return a.Name.Get() < b.Name.Get()
	case "age":
		return a.Age.Get() < b.Age.Get()
	case "created_at":
		return a.CreatedAt.Before(b.CreatedAt)
	case "updated_at":
		return a.UpdatedAt.Before(b.UpdatedAt)
	default:
		return false
	}
}

func paginationBounds(offsetAcc, limitAcc gonest.Accessor[*int64], total int64) (offset, limit int64) {
	offset = 0
	limit = total
	if offsetAcc.IsDirty() {
		if v := offsetAcc.Get(); v != nil {
			offset = *v
		}
	}
	if limitAcc.IsDirty() {
		if v := limitAcc.Get(); v != nil {
			limit = offset + *v
		}
	}
	if offset > total {
		offset = total
	}
	if limit > total {
		limit = total
	}
	if limit < offset {
		limit = offset
	}
	return offset, limit
}

var Provider = gonest.NewProvider(func(provider *gonest.Provider) {
	provider.Constructor(func() *Service { return &Service{} })
})
