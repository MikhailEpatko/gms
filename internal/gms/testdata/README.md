# Заготовки ответов GitLab

Тесты `internal/gms` подставляют эти файлы в ответы фейкового GitLab из `helpers_test.go`: узел `lastDeployment` окружения и содержимое `application.version` на коммите деплоя. Реальные ответы собраны в `testGitLab`, синтетические тест подставляет поверх него через `withDeployment` и `withVersion`. Поля урезаны до тех, что читает `gms`.

## Реальные ответы

`project1/` и `project2/` сняты с GraphQL GitLab 2026-10-18 для `group/project1` и `group/project2`. У всех окружений project2 один коммит деплоя (`1abfac43`), поэтому `application.version` у него один. У окружений project1 коммиты разные, и на каждый свой `application.version`.

## Синтетические

`synthetic/` написаны вручную: `last-deployment-*` — правкой реального ответа, `application.version-*` — с нуля.

| Файл | Из чего собран |
|---|---|
| `last-deployment-no-job.json` | `project2/last-deployment-dev.json` с `job: null` |
| `last-deployment-no-sha.json` | `project2/last-deployment-dev.json` без `commit.sha` |
| `last-deployment-title-html.json` | то же, заголовок `Fix <login> & retry` |
| `application.version-no-build-version` | одна строка `version=7.0.2` без `build_version` |
| `application.version-empty-build-version` | одна строка `build_version=` без значения |
