package main

type Task struct {
	id     int
	cmd    string
	cancel func()
}

type TaskPool struct {
	tasks map[int]Task
}

func (p *TaskPool) CancelTask(id int) {
	for i := range p.tasks {
		if p.tasks[i].id == id {
			p.tasks[i].cancel()
		}
	}
}

var taskPool TaskPool
