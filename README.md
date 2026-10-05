# Умная вентиляция

Стартовая микросервисная архитектура: Expo → Caddy → Go gateway → devices → EMQX → ESP32;
ESP32 → EMQX → telemetry → PostgreSQL + Redis. Сервисы запускаются отдельными процессами из общего Go-модуля.
Каталог `bakend` сохранён по запросу.

## Запуск

```sh
cp .env.example .env
docker compose up -d --build
curl http://localhost:8080/healthz
curl -X POST http://localhost:8080/api/devices -H 'Content-Type: application/json' -d '{"id":"damper-001","name":"Заслонка"}'
curl -X POST http://localhost:8080/api/devices/damper-001/position -H 'Content-Type: application/json' -d '{"position":50}'
curl http://localhost:8080/api/telemetry/damper-001
```

EMQX dashboard: http://localhost:18083 (первый вход admin/public; сменить пароль).
MQTT: порт 1883 доступен устройствам в локальной сети. PostgreSQL и Redis доступны только внутри Compose.
`docker compose down` сохраняет данные; `down -v` удаляет их.
SQL инициализация применяется только на пустом PostgreSQL volume. Последующие изменения схемы требуют миграций.

## Expo

```sh
cd application
cp .env.example .env
npm ci
npm start
```

В EXPO_PUBLIC_API_URL укажите LAN IP компьютера, доступный телефону. localhost подходит только для запуска на самом компьютере.
Для Android-эмулятора используйте 10.0.2.2. Стартовое приложение использует Expo SDK 55.

## ESP32 / PlatformIO

```sh
cd iots/damper
cp include/secrets.example.h include/secrets.h
# Заполнить Wi-Fi, MQTT_HOST (LAN IP), DEVICE_ID и SERVO_PIN
pio run
pio run -t upload
pio device monitor
```

Выбраны ESP32 DevKit и сервопривод на GPIO18. Сервоприводу нужен подходящий внешний источник питания и общая земля.
Диапазон 0–180° необходимо откалибровать под механизм. Перед испытанием отключить механическую нагрузку.
Позиция в телеметрии — заданная позиция сервопривода, без датчика обратной связи.

## Проверки

```sh
cd bakend && go test ./... && go vet ./...
cd ../application && npm run typecheck
```

Подробности контрактов и ограничений: [docs/architecture.md](docs/architecture.md).

Caddy — внешняя REST-точка на http://localhost:8080. Gateway не публикует порт на хост. Expo использует LAN IP с тем же портом. Настройки: infra/caddy/Caddyfile. Для production указать домен, открыть 80/443 и настроить HTTPS.

## Текущий локальный стенд

Окружение запущено: REST http://localhost:8080, EMQX dashboard http://localhost:18084, MQTT localhost:1884. Порт 1883 был занят другим проектом; значения сохранены в локальном .env. Для прошивки укажите MQTT_PORT 1884 и LAN IP компьютера.

Проверены Go build/test/vet, TypeScript, совместимость Expo-зависимостей, сборка PlatformIO и сквозной маршрут Caddy → REST → MQTT → PostgreSQL/Redis → REST с симулятором устройства. Реальное устройство не подключалось.

npm audit: uuid обновлён до 11.1.1 через override для xcode; устранены 6 moderate замечаний. Остаются 21 high замечание из двух первопричин (braces и node-forge), для которых опубликованных исправлений пока нет. Подробности: [docs/dependency-security.md](docs/dependency-security.md).
