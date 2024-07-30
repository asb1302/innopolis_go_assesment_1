package app

import (
	"context"
	"github.com/asb1302/innopolis_go_assesment_1/internal/repository"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/asb1302/innopolis_go_assesment_1/internal/config"
	"github.com/asb1302/innopolis_go_assesment_1/internal/types"
)

type App struct {
	cfg         *config.Config
	cache       map[string][]types.Message
	channels    map[string]chan types.Message
	queue       chan types.Message
	mutex       sync.Mutex
	wg          sync.WaitGroup
	workerCount map[string]int
	writer      types.FileWriter
	userRepo    *repository.UserRepository
}

func NewApp(cfg *config.Config, writer types.FileWriter, userRepo *repository.UserRepository) *App {
	return &App{
		cfg:         cfg,
		cache:       make(map[string][]types.Message),
		channels:    make(map[string]chan types.Message),
		queue:       make(chan types.Message, 1000),
		workerCount: make(map[string]int),
		writer:      writer,
		userRepo:    userRepo,
	}
}

func (a *App) Start(ctx context.Context) {
	log.Println("запуск приложения")

	a.wg.Add(1)
	go a.processQueue(ctx)

	for fileID, ch := range a.channels {
		for i := 0; i < a.cfg.NumWorkers; i++ {
			a.wg.Add(1)
			go a.writeMsgsToCache(ctx, ch)
			a.workerCount[fileID]++
			log.Printf("запущен обработчик сообщений для файла: %s, воркер: %d", fileID, i)
		}
	}

	a.wg.Add(1)
	go a.writeFiles(ctx)

	<-ctx.Done()

	a.wg.Wait()

	log.Println("завершение приложения")
}

func (a *App) processQueue(ctx context.Context) {
	defer a.wg.Done()
	for {
		select {
		case msg := <-a.queue:
			a.mutex.Lock()

			if ch, exists := a.channels[msg.FileID]; exists {
				select {
				case ch <- msg:
				default:
					log.Printf("Канал для файла %s переполнен, добавляем воркера", msg.FileID)
					a.wg.Add(1)
					go a.writeMsgsToCache(ctx, ch)
					a.workerCount[msg.FileID]++
					ch <- msg
				}
			} else {
				log.Printf("канал для файла %s не существует", msg.FileID)
			}

			a.mutex.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func (a *App) writeMsgsToCache(ctx context.Context, ch <-chan types.Message) {
	defer a.wg.Done()
	for {
		select {
		case msg := <-ch:
			log.Printf("Получено сообщение для кеширования: %v", msg)
			a.mutex.Lock()
			a.cache[msg.FileID] = append(a.cache[msg.FileID], msg)
			a.mutex.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func (a *App) writeFiles(ctx context.Context) {
	defer a.wg.Done()
	ticker := time.NewTicker(a.cfg.WorkerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.processCache()
		case <-ctx.Done():
			a.processCache() // Очищаем кэш при завершении работы
			return
		}
	}
}

func (a *App) processCache() {
	log.Println("обработка кэша")

	a.mutex.Lock()
	defer a.mutex.Unlock()

	for fileID, messages := range a.cache {
		if len(messages) == 0 {
			continue
		}

		tempMessages := append([]types.Message{}, messages...)
		a.cache[fileID] = a.cache[fileID][:0] // Очищаем текущий кэш, сохраняя выделенную память

		filePath := filepath.Join(a.cfg.FilesDir, fileID+".txt")

		for attempt := 1; attempt <= a.cfg.MaxRetries; attempt++ {
			if err := a.writer.WriteToFile(filePath, tempMessages); err != nil {
				log.Printf("ошибка при записи в файл %s: %v (попытка %d/%d)\n", filePath, err, attempt, a.cfg.MaxRetries)
				time.Sleep(a.cfg.RetryInterval)
			} else {
				log.Printf("Файл %s успешно записан и кэш очищен", filePath)
				break
			}
		}

		// Если появились новые сообщения для этого файла, добавляем их обратно в кэш
		if len(a.cache[fileID]) > 0 {
			a.cache[fileID] = append(a.cache[fileID], tempMessages...)
		}
	}
}

func (a *App) AddUser(user types.User) error {
	err := a.userRepo.AddUser(user)
	if err != nil {
		return err
	}

	a.mutex.Lock()
	defer a.mutex.Unlock()

	// добавление нового пользователя и создание нового канала для соответствующего файла, если такой канал еще не существует
	if _, exists := a.channels[user.FileID]; !exists {
		a.channels[user.FileID] = make(chan types.Message, 1000)
		log.Printf("Создан канал для файла: %s", user.FileID)

		for i := 0; i < a.cfg.NumWorkers; i++ {
			a.wg.Add(1)
			go a.writeMsgsToCache(context.Background(), a.channels[user.FileID])
			a.workerCount[user.FileID]++
			log.Printf("запущен обработчик сообщений для файла: %s, воркер: %d", user.FileID, i)
		}
	}

	log.Printf("пользователь добавлен: %v", user)

	return nil
}

func (a *App) GetFileCh(fileID string) (chan types.Message, bool) {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	ch, exists := a.channels[fileID]
	return ch, exists
}

func (a *App) SendMsg(msg types.Message) {
	log.Printf("отправка сообщения в очередь: %v", msg)
	a.queue <- msg
}

func (a *App) Shutdown() {
	log.Println("завершение работы, обработка оставшихся сообщений в кэше")
	a.processCache()
	log.Println("завершение работы, кэш обработан")
}

func (a *App) GetWorkerCount(fileID string) int {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	return a.workerCount[fileID]
}
