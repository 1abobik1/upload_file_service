# Привет, вот краткая инструкция по запуску

### Для запуска основного сервера используй команду
```bash
docker-compose up
```

### Вот репозиторий с автогенерированными protobuf-файлами, который использует мой сервер
[https://github.com/1abobik1/proto-upload-service](https://github.com/1abobik1/proto-upload-service)

### Интеграционные тесты находятся в папке `tests/integration`. Для их запуска используй команду (примечание: для Windows используй консоль Git Bash)
```bash
TEST_RUN_ID=$(date +%s) docker-compose -f tests/integration/docker-compose.test.yml up --build
```

### Также я написал клиента для работы с API, чтобы было легче понять как взаимодействовать:
Команды для запуска клиента
```bash
cd .\client\example\
go run main.go -server localhost:50051 -file ./test_photo.jpg -insecure
```
После флага `-file` указывается путь до файла, который ты хочешь загрузить.
```
