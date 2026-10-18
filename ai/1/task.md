Задача 1:

Нужно сделать на Go cli утилиту `gms` со следующим функционалом:

1. Запуск утилиты выполняется командой `gms` с флагами или без.

2. При запуске без параметров утилита выводит полную статистику по всем проектам.

3. Какие могут быть параметры:

- `-p project1 project3 project2` - коды проектов, по которым нужно вывести статистику.
Соответствие кодов проектам нужно задавать в файле конфигурации утилиты.
Хранить конфиг в JSON-файле:
```json
{
  "project1": {
    "DEV": "https://gitlab/project1/-/tree/dev",
    "DEMO": "https://gitlab/project1/-/tree/demo",
    "PROD": "https://gitlab/project1/-/tree/main"
  },
  "project4": {
    "DEV": "https://gitlab/project4/-/tree/dev",
    "DEMO": "https://gitlab/project4/-/tree/demo",
    "PROD": "https://gitlab/project4/-/tree/main"
  },
  "project3": {
    "DEV": "https://gitlab/project3/-/tree/dev",
    "DEMO": "https://gitlab/project3/-/tree/demo",
    "PROD": "https://gitlab/project3/-/tree/main"
  },
  "project2": {
    "DEV": "https://gitlab/project2/-/tree/dev",
    "DEMO": "https://gitlab/project2/-/tree/demo",
    "PROD": "https://gitlab/project2/-/tree/main"
  }
}
```

- `-c` - вывести в терминал путь к файлу конфига.

3. Какую статистику должна выводить утилита:
```
<Код проекта: project1, project4, project3, project2>
DEV:
<дата деплоя>
<версия приложения>
<номер последнего коммита>

DEMO:
<дата деплоя>
<версия приложения>
<номер последнего коммита>

PROD:
<дата деплоя>
<версия приложения>
<номер последнего коммита>
```
Как приложение находит статистику (на примере проекта project2 и окружения DEV):
На странице @https://gitlab.example.com/group/project2/-/jobs?kind=BUILD есть список jobs.
В списке нужно найти последнюю по дате job, у которой:
- refName: "dev"
- stage.name: "deploy"
- status: "SUCCESS"
- finishedAt: "2026-09-24T13:35:33Z" // последняя дата

Пример такой job: @https://gitlab.example.com/group/project2/-/jobs/1830909 
В конце лога этого шага есть записи:
```
Release "project2-dev" has been upgraded. Happy Helming!
NAME: project2-dev
LAST DEPLOYED: Thu Sep 24 16:35:02 2026
NAMESPACE: group-dev
STATUS: deployed
REVISION: 135
TEST SUITE: None
Running time 31.33 seconds
Running after_script
00:00
Running after script...
$ rm -drf /tmp/$CI_JOB_ID
Cleaning up project directory and file based variables
00:00
Job succeeded
```

- <дата деплоя> - соответствует `finishedAt: "2026-09-24T13:35:33Z"`;
- <версия приложения> - хранится в файле @https://gitlab.example.com/group/project2/-/blob/dev/application.version
и соответствует строке `build_version=7.0.2`;
- <номер последнего коммита> - нужно проанализировать конфигурацию ci в @/Users/m.epatko/IdeaProjects/project2/.gitlab-ci.yml,
@/Users/m.epatko/IdeaProjects/project2/.helm и лог job: @https://gitlab.example.com/group/project2/-/jobs/1830909 и понять, 
как достать номер последнего коммита в ветке dev на момент деплоя. Если это невозможно, то не будем выводить <номер последнего коммита>.