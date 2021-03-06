package main

import (
	"encoding/json"
	"sync"

	_ "github.com/joho/godotenv/autoload"
	"github.com/kintoproj/go-utils/config"
	"github.com/kintoproj/go-utils/klog"
	"github.com/kintoproj/go-utils/task"
	"github.com/kintoproj/go-utils/task/machinery"
	"gopkg.in/errgo.v2/fmt/errors"
)

func main() {
	klog.InitLogger()
	client := machinery.NewMachineryTaskClient(&machinery.MachineryConfig{
		BrokerConnectionUri:        config.GetStringOrDie("MACHINERY_REDIS_CONNECTION_URI"),
		DefaultQueueName:           "example-tasks",
		ResultBackendConnectionUri: config.GetString("MACHINERY_MONGODB_HOST", ""),
		ResultsExpireInSeconds:     config.GetInt("MACHINERY_MONGODB_EXPIRE_TIME_SECONDS", 3600),
		WorkersEnabled:             true,
		WorkerAlias:                "example-tasks-workers",
		WorkerConcurrencyLimit:     config.GetInt("MACHINERY_WORKER_CONCURRENCY_LIMIT", 0),
		MaxRetryCount:              config.GetInt("MACHINERY_MAX_RETRY_COUNT", -1),
		RetryTimeoutSeconds:        config.GetInt("MACHINERY_RETRY_TIMEOUT_SECONDS", 0),
	})

	helloWorldClient := HelloWorldTaskClient{client: client}

	const errorsToFakeCount = 3
	errorCount := 0
	wg := sync.WaitGroup{}
	wg.Add(3)

	// Register A Worker/Handler.
	err := helloWorldClient.RegisterHelloWorldWorker(func(task *HelloWorldTask) error {
		klog.Infof("received hello world msg: %s", task.Msg)

		if errorCount <= errorsToFakeCount {
			errorCount++
			return errors.Newf("faking an error to show how retries work. err cnt %d", errorCount)
		}

		wg.Done()
		return nil
	})

	if err != nil {
		klog.PanicfWithError(err, "error registering worker")
	}

	// Submit a Task for the worker(s)
	err = helloWorldClient.SubmitHelloWorldTask(&HelloWorldTask{
		Msg: "yo",
	})

	if err != nil {
		klog.PanicfWithError(err, "error submitting hello task")
	}
	// An example of registering a task that will follow up with another task
	// (does not need to be called chain task worker), could be RegisterSendEmailValidationWorker
	// In our case we would need to chain CreateStripeCustomer -> Create Subscription -> Update Account
	helloWorldClient.RegisterChainTaskWorker(func(task *HelloWorldTask) (*HelloWorldTask, error) {
		klog.Infof("chain task received msg: %s", task.Msg)
		wg.Done()
		return &HelloWorldTask{
			Msg: "hello again!",
		}, nil
	})

	// Submit chaintask. Would be called SubmitRegisterStripeTask for example
	err = helloWorldClient.SubmitHelloChain(&HelloWorldTask{
		Msg: "yo",
	})

	if err != nil {
		klog.PanicfWithError(err, "error registering hello chain")
	}

	wg.Wait()

	klog.Info("jobs done :)")

}

/// Everything below this line would be in its own file....
// Imagine this as a BillingTaskClient, AnalyticsTaskClient, StripeTaskClient, etc

// types.go
// Each task would have its own struct
type HelloWorldTask struct {
	Msg string `json:"msg"`
}

// helloworldtask.go
type HelloWorldTaskClient struct {
	client task.TaskClientInterface
}

func (t *HelloWorldTaskClient) RegisterChainTaskWorker(workerHandler func(task *HelloWorldTask) (*HelloWorldTask, error)) error {
	return t.client.RegisterChainTaskHandler("chaintask", func(data string) (*task.Task, error) {
		t := new(HelloWorldTask)
		err := json.Unmarshal([]byte(data), t)

		if err != nil {
			return nil, err
		}

		nextTask, err := workerHandler(t)

		if err != nil {
			return nil, err
		}

		// Return task to go back into the queue
		return task.NewTask("helloworld", nextTask)
	})
}

func (t *HelloWorldTaskClient) RegisterHelloWorldWorker(workerHandler func(task *HelloWorldTask) error) error {
	return t.client.RegisterTaskHandler("helloworld", func(data string) error {
		task := new(HelloWorldTask)
		err := json.Unmarshal([]byte(data), task)

		if err != nil {
			return err
		}

		return workerHandler(task)
	})
}

func (h *HelloWorldTaskClient) SubmitHelloWorldTask(t *HelloWorldTask) error {
	task, err := task.NewTask("helloworld", t)

	if err != nil {
		return err
	}

	return h.client.SubmitTask(task)
}

func (h *HelloWorldTaskClient) SubmitHelloChain(t *HelloWorldTask) error {
	task, err := task.NewTask("chaintask", t)

	if err != nil {
		return err
	}

	return h.client.SubmitTask(task)
}
