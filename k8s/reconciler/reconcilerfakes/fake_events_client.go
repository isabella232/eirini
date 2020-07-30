// Code generated by counterfeiter. DO NOT EDIT.
package reconcilerfakes

import (
	"context"
	"sync"

	"code.cloudfoundry.org/eirini/k8s/reconciler"
	v1 "k8s.io/api/core/v1"
	v1a "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FakeEventsClient struct {
	CreateStub        func(context.Context, *v1.Event, v1a.CreateOptions) (*v1.Event, error)
	createMutex       sync.RWMutex
	createArgsForCall []struct {
		arg1 context.Context
		arg2 *v1.Event
		arg3 v1a.CreateOptions
	}
	createReturns struct {
		result1 *v1.Event
		result2 error
	}
	createReturnsOnCall map[int]struct {
		result1 *v1.Event
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeEventsClient) Create(arg1 context.Context, arg2 *v1.Event, arg3 v1a.CreateOptions) (*v1.Event, error) {
	fake.createMutex.Lock()
	ret, specificReturn := fake.createReturnsOnCall[len(fake.createArgsForCall)]
	fake.createArgsForCall = append(fake.createArgsForCall, struct {
		arg1 context.Context
		arg2 *v1.Event
		arg3 v1a.CreateOptions
	}{arg1, arg2, arg3})
	fake.recordInvocation("Create", []interface{}{arg1, arg2, arg3})
	fake.createMutex.Unlock()
	if fake.CreateStub != nil {
		return fake.CreateStub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	fakeReturns := fake.createReturns
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeEventsClient) CreateCallCount() int {
	fake.createMutex.RLock()
	defer fake.createMutex.RUnlock()
	return len(fake.createArgsForCall)
}

func (fake *FakeEventsClient) CreateCalls(stub func(context.Context, *v1.Event, v1a.CreateOptions) (*v1.Event, error)) {
	fake.createMutex.Lock()
	defer fake.createMutex.Unlock()
	fake.CreateStub = stub
}

func (fake *FakeEventsClient) CreateArgsForCall(i int) (context.Context, *v1.Event, v1a.CreateOptions) {
	fake.createMutex.RLock()
	defer fake.createMutex.RUnlock()
	argsForCall := fake.createArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeEventsClient) CreateReturns(result1 *v1.Event, result2 error) {
	fake.createMutex.Lock()
	defer fake.createMutex.Unlock()
	fake.CreateStub = nil
	fake.createReturns = struct {
		result1 *v1.Event
		result2 error
	}{result1, result2}
}

func (fake *FakeEventsClient) CreateReturnsOnCall(i int, result1 *v1.Event, result2 error) {
	fake.createMutex.Lock()
	defer fake.createMutex.Unlock()
	fake.CreateStub = nil
	if fake.createReturnsOnCall == nil {
		fake.createReturnsOnCall = make(map[int]struct {
			result1 *v1.Event
			result2 error
		})
	}
	fake.createReturnsOnCall[i] = struct {
		result1 *v1.Event
		result2 error
	}{result1, result2}
}

func (fake *FakeEventsClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.createMutex.RLock()
	defer fake.createMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeEventsClient) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ reconciler.EventsClient = new(FakeEventsClient)