package bucketworker

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
	bucketIdx int
}

// Worker represents the worker that executes the job
type worker struct {
	jobChannel chan job
	quit       chan bool
}

func newWorker(maxJobs int) worker {
	return worker{
		jobChannel: make(chan job, maxJobs),
		quit:       make(chan bool)}
}

func (w worker) start() {
	go func() {
		for {
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

type BucketDispatcher struct {
	jobQueue     *list.List
	jobQueueMut  sync.Mutex
	jobQueueCond sync.Cond

	workers    []worker
	maxWorkers int
	callbackC  chan func()
}

func NewBucketDispatcher(maxJobs, maxWorkers int, callbackC chan func()) *BucketDispatcher {
	workers := make([]worker, maxWorkers)
	for i := 0; i < maxWorkers; i++ {
		workers[i] = newWorker(maxJobs)
	}
	d := &BucketDispatcher{
		jobQueue:   list.New(),
		workers:    workers,
		maxWorkers: maxWorkers,
		callbackC:  callbackC,
	}
	d.jobQueueCond.L = &d.jobQueueMut
	return d
}

func (d *BucketDispatcher) Run() {
	// starting n number of workers
	for i := 0; i < d.maxWorkers; i++ {
		d.workers[i].start()
	}

	go d.dispatch()
}

func (d *BucketDispatcher) dispatch() {
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

		idx := j.bucketIdx
		jobChannel := d.workers[idx].jobChannel

		// dispatch the job to the worker job channel
		jobChannel <- j
	}
}

func (d *BucketDispatcher) Exec(execFunc func() []byte, callbackFunc func([]byte), bucket int) {
	j := job{
		payload: payload{
			execFunc:     execFunc,
			callbackFunc: callbackFunc,
		},
		callbackC: d.callbackC,
		bucketIdx: bucket,
	}
	d.jobQueueMut.Lock()
	d.jobQueue.PushBack(j)
	d.jobQueueCond.Broadcast()
	d.jobQueueMut.Unlock()
}
