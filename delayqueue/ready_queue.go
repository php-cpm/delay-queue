package delayqueue

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/php-cpm/delay-queue/config"
)

// 添加JobId到队列中
// 将消息放到 redis
// http://www.redis.cn/commands/rpush.html
// queueName:Topic
func pushToReadyQueue(queueName string, jobId string) error {
	queueName = fmt.Sprintf(config.Setting.QueueName, queueName)
	_, err := execRedisCommand("RPUSH", queueName, jobId)

	return err
}

// 从队列中阻塞获取JobId
func blockPopFromReadyQueue(queues []string, timeout int) (string, error) {
	// 	var args []interface{}
	// 	args = append(args, queue)
	// }
	// args = append(args, timeout)
	var value interface{}
	var err error
	t := time.Now().Unix() + int64(timeout)
A:
	for time.Now().Unix() < t {
		for _, queue := range queues {
			queue = fmt.Sprintf(config.Setting.QueueName, queue)
			value, err = execRedisCommand("LPOP", queue) //使用codis,去掉blpop命令
			if err != nil {
				return "", err
			}
			if value != nil {
				break A
			}
		}
		sleepTimeInterval()
	}
	if value == nil {
		return "", nil
	}
	// var valueBytes []interface{}
	// valueBytes = value.([]interface{})
	// if len(valueBytes) == 0 {
	// 	return "", nil
	// }
	element := string(value.([]byte))

	return element, nil
}

// 从队列中阻塞获取JobId,blpop
func blPopFromReadyQueue(queues []string, timeout int) (string, error) {
	var args []interface{}
	for _, queue := range queues {
		queue = fmt.Sprintf(config.Setting.QueueName, queue)
		args = append(args, queue)
	}
	args = append(args, timeout)
	// http://www.redis.cn/commands/blpop.html
	value, err := execRedisCommand("BLPOP", args...)
	if err != nil {
		return "", err
	}
	if value == nil {
		return "", nil
	}
	var valueBytes []interface{}
	valueBytes = value.([]interface{})
	if len(valueBytes) == 0 {
		return "", nil
	}
	element := string(valueBytes[1].([]byte))

	return element, nil
}

// 请求的最小时间间隔(毫秒)
var RetryMinTimeInterval int64 = 5

// 请求的最大时间间隔(毫秒)
var RetryMaxTimeInterval int64 = 30

// sleepTimeInterval 随机休眠一段时间
// 随机时间范围[RetryMinTimeInterval,RetryMaxTimeInterval)
func sleepTimeInterval() {
	var unixNano = time.Now().UnixNano()
	var r = rand.New(rand.NewSource(unixNano))
	var randValue = RetryMinTimeInterval + r.Int63n(RetryMaxTimeInterval-RetryMinTimeInterval)
	time.Sleep(time.Duration(randValue) * time.Millisecond)
}
