package delayqueue

import (
	"github.com/php-cpm/delay-queue/config"
	"github.com/vmihailenco/msgpack"
)

// Job 使用msgpack序列化后保存到Redis,减少内存占用
type Job struct {
	Topic string `json:"topic" msgpack:"1"` // topic ，唯一
	Id    string `json:"id" msgpack:"2"`    // job唯一标识ID 。客户端需要保证唯一性。有关联关系的
	Delay int64  `json:"delay" msgpack:"3"` // 延迟时间, unix时间戳
	TTR   int64  `json:"ttr" msgpack:"4"`   // 超时时间,TTR的设计目的是为了保证消息传输的可靠性。
	Body  string `json:"body" msgpack:"5"`  // body
}

// 获取Job
func getJob(key string) (*Job, error) {
	value, err := execRedisCommand("GET", config.DefaultKeyName+key)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}

	byteValue := value.([]byte)
	job := &Job{}
	err = msgpack.Unmarshal(byteValue, job)
	if err != nil {
		return nil, err
	}

	return job, nil
}

// 添加Job
func putJob(key string, job Job) error {
	value, err := msgpack.Marshal(job)
	if err != nil {
		return err
	}
	_, err = execRedisCommand("SET", config.DefaultKeyName+key, value)

	return err
}

// 删除Job
func removeJob(key string) error {
	_, err := execRedisCommand("DEL", config.DefaultKeyName+key)

	return err
}
