# YTsaurus Excel integration

Includes two microservices
* [Exporter](./exporter) — downloads data from YTsaurus Static tables or YTsaurus QueryTracker results as an Excel spreadsheet
* [Uploader](./uploader) — uploads data from an Excel spreadsheet into a YTsaurus Static tables


Helm Chart

The chart can be used to install both services in a k8s cluster. To do this, you need to specify the configuration of the YT cluster (currently only one cluster is supported) and the services themselves. The logs are persistent and rotated periodically.

```
helm install ytsaurus-excel deployments/ytsaurus-excel-chart/. --values my-values.yaml
``` 

The exporter and uploader configuration is the same only for now, it may change in the future. To make the services functions appear in the UI, you need to set uploadTableExcelBaseUrl and exportTableBaseUrl in [uiSettings](https://github.com/ytsaurus/ytsaurus-ui/blob/00c06919c5a31ea45068c9dbfe3f3ce5e0bbef3b/packages/ui/src/shared/ui-settings.ts#L156C14-L156C30). For example, you can use [chart](https://github.com/ytsaurus/ytsaurus-ui/tree/00c06919c5a31ea45068c9dbfe3f3ce5e0bbef3b/ytsaurus-ui-chart) to do this.

