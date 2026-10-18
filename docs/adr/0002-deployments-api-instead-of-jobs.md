# Deployments API вместо перебора jobs

В `ai/1/task.md` дата деплоя ищется перебором jobs проекта (`refName`, `stage.name == deploy`, `status == SUCCESS`, максимум `finishedAt`), а версия читается с HEAD ветки. Мы отступили от спеки: во всех репозиториях deploy-jobs объявлены с `environment: name: dev|demo|prod`, поэтому текущий деплой берётся одним запросом к Deployments API (`environment=<окружение в нижнем регистре>`, `status=success`). Оттуда же берутся коммит деплоя (`sha`, `commit.title`) и дата (`deployable.finished_at`), а `application.version` читается на коммите деплоя.

## Consequences

- Текущий деплой учитывается из любой ветки. В DEV регулярно деплоят из MR (ref `refs/merge-requests/<N>/head`), и такой деплой будет показан с пометкой `(MR !<N>)`, а не пропущен.
- DEMO и PROD деплоятся вручную, поэтому HEAD ветки может опережать стенд. Версия, прочитанная на коммите деплоя, этого расхождения не даёт.
- Если в CI какого-то проекта у deploy-job не будет `environment`, утилита покажет для него `нет деплоев`.
