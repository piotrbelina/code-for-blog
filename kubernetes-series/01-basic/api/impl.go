package api

import (
	"context"
	"sync"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

var _ StrictServerInterface = (*TodoStore)(nil)

type TodoStore struct {
	todos map[openapi_types.UUID]Todo
	Lock  sync.Mutex
}

func NewTodoStore() *TodoStore {
	return &TodoStore{
		todos: make(map[openapi_types.UUID]Todo),
	}
}

func (t *TodoStore) GetTodos(ctx context.Context, r GetTodosRequestObject) (GetTodosResponseObject, error) {
	t.Lock.Lock()
	defer t.Lock.Unlock()

	var result []Todo

	for _, todo := range t.todos {
		result = append(result, todo)
	}

	return GetTodos200JSONResponse(result), nil
}

func (t *TodoStore) PostTodos(ctx context.Context, r PostTodosRequestObject) (PostTodosResponseObject, error) {
	t.Lock.Lock()
	defer t.Lock.Unlock()

	title := r.Body.Title

	if title == "" {
		return PostTodos400Response{}, nil
	}

	id := uuid.New()
	todo := Todo{
		Id:    &id,
		Title: title,
	}

	t.todos[id] = todo

	return PostTodos201JSONResponse(todo), nil
}

func (t *TodoStore) GetTodosId(ctx context.Context, r GetTodosIdRequestObject) (GetTodosIdResponseObject, error) {
	t.Lock.Lock()
	defer t.Lock.Unlock()

	todo, ok := t.todos[r.Id]
	if !ok {
		return GetTodosId404Response{}, nil
	}
	return GetTodosId200JSONResponse(todo), nil
}

func (t *TodoStore) DeleteTodosId(ctx context.Context, r DeleteTodosIdRequestObject) (DeleteTodosIdResponseObject, error) {
	_, ok := t.todos[r.Id]
	if !ok {
		return DeleteTodosId404Response{}, nil
	}
	delete(t.todos, r.Id)

	return DeleteTodosId200Response{}, nil
}

func (t *TodoStore) PutTodosId(ctx context.Context, r PutTodosIdRequestObject) (PutTodosIdResponseObject, error) {
	todo, ok := t.todos[r.Id]
	if !ok {
		return PutTodosId404Response{}, nil
	}

	title := r.Body.Title
	if title == "" {
		return PutTodosId400Response{}, nil
	}

	todo.Title = title

	t.todos[r.Id] = todo

	return PutTodosId200Response{}, nil
}
