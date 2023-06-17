package main

import (
	"fmt"
	"go.uber.org/zap"
	"sync"
)

type procMap struct {
	lock sync.RWMutex
	m    map[string]*ProcList
}

func (pm *procMap) AddTask(t *Task) error {
	proc := pm.getProc(t)
	if proc == nil {
		Logger.Error("get a nil processor, please check")
		return fmt.Errorf("get a nil processor, please check")
	}
	var err error
	_, err = proc.Exec(`taskID = "%s"`, t.id)
	_, err = proc.Exec(`source("./rscript/%s.R")`, t.name)
	if err != nil {
		Logger.Error("Exec failed, err: ", err)
		return err
	}
	return nil
}

func (pm *procMap) TaskComplete(taskName, taskID string, kill bool) {
	// TODO collect result
	pm.lock.Lock()
	defer pm.lock.Unlock()
	pList := pm.m[taskName]
	for i := pList.Back(); i != nil; i = i.Prev() {
		proc := i.Value.(*Proc)
		if proc.task != nil && proc.task.id == taskID {
			Logger.Infow("Task complete success", zap.String("taskName", taskName), zap.String("taskID", taskID))
			proc.task = nil
			if kill {
				_ = proc.Close()
				pList.Remove(i)
			}
			return
		}
	}
}

// bind task with processor
func (pm *procMap) getProc(t *Task) *Proc {
	pm.lock.Lock()
	defer pm.lock.Unlock()
	pList := pm.m[t.name]
	// create new pList and processor
	if pList == nil || pList.Len() == 0 {
		proc := pm.makeNewProc(t.name)
		proc.task = t
		return proc
	}

	// try to get an idle processor
	for procElement := pList.Front(); procElement != nil; procElement = procElement.Next() {
		proc := procElement.Value.(*Proc)
		if proc != nil && proc.task == nil {
			Logger.Infow("Find an idle processor", zap.String("taskName", t.name), zap.String("taskID", t.id))
			proc.task = t
			// put the procElement to the end of list
			pList.MoveToBack(procElement)
			return proc
		}
	}

	// can not find an idle processor
	// create a new processor
	proc := pm.makeNewProc(t.name)
	proc.task = t
	return proc
}

// This func is in order to reduce lock granularity
func (pm *procMap) makeNewProc(name string) *Proc {
	pm.lock.Unlock()
	defer pm.lock.Lock()
	return newProc(name)
}
