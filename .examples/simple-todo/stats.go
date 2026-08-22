package main

// TodoStats is the aggregate returned by GET /todos/stats.
type TodoStats struct {
	Total int `json:"total"`
	Done  int `json:"done"`
}

// TodoStatsUsecase is a per-route dependency, deliberately built directly
// from a *TodoService obtained via gonest.MustInject[*TodoService](route) --
// see controller.go's GET /todos/stats route -- rather than registered as its
// own Provider: TodoService is a Singleton, and Provider-to-Provider
// MustInject copies the resolved instance into a placeholder ONCE, at
// bootstrap time (see internal/inject's own PendingEdge doc comment), which
// would silently stop tracking TodoService's own later mutations (its items
// slice grows via appends after every POST). Resolving directly from the
// route with gonest.MustInject[*TodoService](route) instead hands back the
// SAME already-fully-resolved singleton pointer TodoController itself holds
// -- no copy, no staleness -- which is exactly what route-must-inject is for.
type TodoStatsUsecase struct {
	service *TodoService
}

func (u *TodoStatsUsecase) Run() *TodoStats {
	items := u.service.List()
	stats := &TodoStats{Total: len(items)}
	for _, t := range items {
		if t.Done {
			stats.Done++
		}
	}
	return stats
}
