package internal

import (
	"errors"
	"fmt"
	"sync"
)

// Task is a running command, which can be terminated with cancel()
type Task struct {
	Cmd    string
	Cancel func()
}

const maxTaskID = 200000

// TaskPool ...
type TaskPool struct {
	MaxRunningTasks int
	tasks           map[int]Task
	mu              sync.RWMutex
	lastAddedId     int
}

func NewTaskPool(maxRunningTasks int) *TaskPool {
	return &TaskPool{
		MaxRunningTasks: maxRunningTasks,
		tasks:           make(map[int]Task, maxRunningTasks),
		lastAddedId:     -1,
	}
}

func (p *TaskPool) Add(cmd string, cancel func()) (newId int, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := p.lastAddedId + 1
	reachedMaxId := false
	for {
		if _, exists := p.tasks[id]; !exists {
			break
		}
		id++
		if id >= maxTaskID {
			if !reachedMaxId {
				id = 0
				reachedMaxId = true
			} else {
				return 0, errors.New("reached max task id limit")
			}
		}
	}
	p.tasks[id] = Task{
		Cmd:    cmd,
		Cancel: cancel,
	}
	p.lastAddedId = id
	return id, nil
}

func (p *TaskPool) Cancel(id int) (ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if v, exists := p.tasks[id]; exists {
		v.Cancel()
		delete(p.tasks, id)
		return true
	} else {
		return false
	}
}

func (p *TaskPool) List() (taskDescriptions []string) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for i, v := range p.tasks {
		taskDescriptions = append(taskDescriptions,
			fmt.Sprintf("[%d]: %s", i, v.Cmd))
	}
	return
}
