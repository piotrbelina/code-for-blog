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

	result := make([]Todo, 0)

	for _, todo := range t.todos {
		result = append(result, todo)
	}

	return GetTodos200JSONResponse(result), nil
}

func (t *TodoStore) PostTodo(ctx context.Context, r PostTodoRequestObject) (PostTodoResponseObject, error) {
	t.Lock.Lock()
	defer t.Lock.Unlock()

	title := r.Body.Title

	if title == "" {
		return PostTodo400Response{}, nil
	}

	id := uuid.New()
	todo := Todo{
		Id:    &id,
		Title: title,
	}

	t.todos[id] = todo

	return PostTodo201JSONResponse(todo), nil
}

func (t *TodoStore) GetTodo(ctx context.Context, r GetTodoRequestObject) (GetTodoResponseObject, error) {
	t.Lock.Lock()
	defer t.Lock.Unlock()

	todo, ok := t.todos[r.Id]
	if !ok {
		return GetTodo404Response{}, nil
	}
	return GetTodo200JSONResponse(todo), nil
}

func (t *TodoStore) DeleteTodo(ctx context.Context, r DeleteTodoRequestObject) (DeleteTodoResponseObject, error) {
	_, ok := t.todos[r.Id]
	if !ok {
		return DeleteTodo404Response{}, nil
	}
	delete(t.todos, r.Id)

	return DeleteTodo200Response{}, nil
}

func (t *TodoStore) PutTodo(ctx context.Context, r PutTodoRequestObject) (PutTodoResponseObject, error) {
	todo, ok := t.todos[r.Id]
	if !ok {
		return PutTodo404Response{}, nil
	}

	title := r.Body.Title
	if title == "" {
		return PutTodo400Response{}, nil
	}

	todo.Title = title

	t.todos[r.Id] = todo

	return PutTodo200Response{}, nil
}
