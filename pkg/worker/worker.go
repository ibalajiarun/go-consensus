package worker

import (
	"container/list"
	"sync"
)

type payload struct {
	execFunc     func() []byte
	callbackFunc func([]byte)
}

// Job represents the job to be run
type job struct {
	payload   payload
	callbackC chan func()
}

// Worker represents the worker that executes the job
type worker struct {
	workerPool chan chan job
	jobChannel chan job
	quit       chan bool
}

func newWorker(workerPool chan chan job) worker {
	return worker{
		workerPool: workerPool,
		jobChannel: make(chan job),
		quit:       make(chan bool)}
}

// start method starts the run loop for the worker, listening for a quit channel in
// case we need to stop it
func (w worker) start() {
	go func() {
		for {
			// register the current worker into the worker queue.
			w.workerPool <- w.jobChannel

			select {
			case job := <-w.jobChannel:
				result := job.payload.execFunc()
				job.callbackC <- func() { job.payload.callbackFunc(result) }

			case <-w.quit:
				// we have received a signal to stop
				return
			}
		}
	}()
}

// Stop signals the worker to stop listening for work requests.
func (w worker) Stop() {
	go func() {
		w.quit <- true
	}()
}

type Dispatcher struct {
	// JobQueue is a buffered channel that we can send work requests on.
	// jobQueue chan job
	jobQueue     *list.List
	jobQueueMut  sync.Mutex
	jobQueueCond sync.Cond
	// A pool of workers channels that are registered with the dispatcher
	workerPool chan chan job
	maxWorkers int
	callbackC  chan func()
}

func NewDispatcher(maxJobs, maxWorkers int, callbackC chan func()) *Dispatcher {
	// jobQueue := make(chan job, maxJobs)
	pool := make(chan chan job, maxWorkers)
	d := &Dispatcher{
		jobQueue:   list.New(),
		workerPool: pool,
		maxWorkers: maxWorkers,
		callbackC:  callbackC,
	}
	d.jobQueueCond.L = &d.jobQueueMut
	return d
}

func (d *Dispatcher) Run() {
	// starting n number of workers
	for i := 0; i < d.maxWorkers; i++ {
		worker := newWorker(d.workerPool)
		worker.start()
	}

	go d.dispatch()
}

func (d *Dispatcher) dispatch() {
	for {
		d.jobQueueMut.Lock()
		elem := d.jobQueue.Front()
		for elem == nil {
			d.jobQueueCond.Wait()
			elem = d.jobQueue.Front()
		}
		d.jobQueue.Remove(elem)
		d.jobQueueMut.Unlock()

		j := elem.Value.(job)

		// try to obtain a worker job channel that is available.
		// this will block until a worker is idle
		jobChannel := <-d.workerPool

		// dispatch the job to the worker job channel
		jobChannel <- j
	}
}

func (d *Dispatcher) Exec(execFunc func() []byte, callbackFunc func([]byte)) {
	j := job{
		payload: payload{
			execFunc:     execFunc,
			callbackFunc: callbackFunc,
		},
		callbackC: d.callbackC,
	}
	d.jobQueueMut.Lock()
	d.jobQueue.PushBack(j)
	d.jobQueueCond.Broadcast()
	d.jobQueueMut.Unlock()
}

func (d *Dispatcher) CallbackC() <-chan func() {
	return d.callbackC
}
