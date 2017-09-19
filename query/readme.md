### 1 状态清除接口
---

清除一个节点下，一个监控项/机器的报警状态及block状态。

如果给定host但alarm为空，则清楚该host下所有alarm状态；如果给定alarm但host为空，则清楚该alarm下所有host的状态。如果给定alarm同时给定host，则清除该alarm、host的状态

    # 清除一个监控项的报警状态
    curl "http://127.0.0.1/event/clear/status?ns=server.product.loda&alarm=alarm-version"
    # 清除一个机器的报警状态
    curl "http://127.0.0.1/event/clear/status?ns=server.product.loda&host=hostname"
    # 清除一个监控项中一台机器的报警
    curl "http://127.0.0.1/event/clear/status?ns=server.product.loda&alarm=alarm-version&host=hostname"


### 2 发送通知接口
---

    curl -X POST -d '{"types":["sms","mail"],"subject":"zzz test", "content":"zzz test", "groups":["loda.monitor.event-dev"]}' "http://event.monitor.ifengidc.com/event/output"