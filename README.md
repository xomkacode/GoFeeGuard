# GoFeeGuard

Лёгкая Go-библиотека, которая разрешает запуск приложения до заданной даты. Текущее время сверяется с несколькими HTTPS-источниками. Если срок истёк или время проверить не удалось, приложение может завершиться без сообщения.

## Установка

```sh
go get github.com/xomkacode/GoFeeGuard/feeguard
```

Импорт:

```go
import "github.com/xomkacode/GoFeeGuard/feeguard"
```

## Быстрый старт

Дата указывается в формате `ГГГГ-ММ-ДД`. Последний день включён в срок работы и считается по местному времени компьютера.

```go
package main

import (
	"context"
	"time"

	"github.com/xomkacode/GoFeeGuard/feeguard"
)

func main() {
	clock, err := feeguard.NewNetworkClock()
	if err != nil {
		return
	}

	guard, err := feeguard.New(
		"2027-12-31",
		feeguard.WithTimeSource(clock),
	)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if !guard.Active(ctx) {
		return
	}

	startApplication()
}
```

Замените `startApplication()` запуском своего приложения. `Active` возвращает только `true` или `false`: при окончании срока и при ошибке проверки времени программа просто завершится.

## Источники времени

По умолчанию используются три HTTPS-адреса:

| Источник | Адрес |
| --- | --- |
| Google | `https://www.google.com/generate_204` |
| Cloudflare | `https://www.cloudflare.com/` |
| Wikipedia | `https://www.wikipedia.org/` |

Библиотека читает заголовок ответа `Date`. Для успешной проверки нужны минимум два ответа, совпадающих с точностью до двух минут. Если сеть недоступна или серверы расходятся, запуск блокируется.

## Собственные источники

Можно передать свои HTTPS-адреса. Каждый сервер должен возвращать корректный заголовок `Date`; для независимой проверки используйте разные хосты.

```go
clock, err := feeguard.NewNetworkClock(
	"https://time-a.example.com/",
	"https://time-b.example.net/",
	"https://time-c.example.org/",
)
```

## Ограничения защиты

Проверка нескольких HTTPS-источников усложняет обычную подмену системных часов. Она не защищает от пользователя, который контролирует устройство и может изменить бинарник или сетевой трафик. Для более сильной защиты решение о доступе должно принимать сервер, которым вы управляете.
