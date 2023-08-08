# Content-types

| Header <br/>`Content-Type` | Header <br/>`X-Chart-Type` |          Content            |
|:---------------------------| :-------------------------:| :-------------------------- |
| text/html                  | "echart"                   | Full HTML (echart) <br/>ex) It may be inside of `<iframe>`|
| text/html                  | -                          | Full HTML <br/>ex) It may be inside of `<iframe>` |
| text/csv                   | -                          | CSV                         |
| text/markdown              | -                          | Markdown                    |
| application/json           | "echart"                   | JSON (echart data)          |
| application/json           | -                          | JSON                        |
| application/xhtml+xml      | -                          | HTML Element, ex) `<div>...</div>` |

