package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"
)

// Всего запросов
const totalRequests = 25000

// Скорость отправки запросов (500 запросов/сек.)
const requestsPerSecond = 1000

// Возможные значения для ID
const value1 = "1"
const value2 = "2"
const value3 = "3"

var wg sync.WaitGroup

func main() {
	wg.Add(totalRequests)

	// Будем отправлять по 500 запросов каждую секунду
	ticker := time.Tick(time.Second)

	// Стартуем процесс выполнения запросов
	go func() {
		i := 0
		for t := range ticker {
			if i >= totalRequests {
				break
			}
			for j := 0; j < requestsPerSecond && i < totalRequests; j++ {
				value := value1
				if j%2 == 0 {
					value = value2
				}
				if j%3 == 0 {
					value = value3
				}
				go sendRequest(value)
				i++
			}
			fmt.Printf("[%v]: Выполнено %d запросов.\n", t.Format("15:04:05"), i)
		}
	}()

	wg.Wait()
	fmt.Println("Все запросы завершены.")
}

// Отправляет один запрос с данным ID
func sendRequest(id string) {
	defer wg.Done()

	url := fmt.Sprintf("http://127.0.0.1:80/counter/%s", id)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("Ошибка при отправке запроса: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Чтение тела ответа
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Ошибка при чтении ответа: %v\n", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Неверный статус ответа (%d)\n", resp.StatusCode)
	}
}
