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
    # 清楚一个监控项中一台机器的某中tag的报警
    curl "http://127.0.0.1/event/clear/status?ns=server.product.loda&alarm==alarm-version&host=hostname&tagString=host%3d=alarm-version%3bcpu_num%3d1"

### 2 发送通知接口
---

    curl -X POST -d '{"types":["sms","mail"],"subject":"zzz test", "content":"zzz test", "groups":["loda.monitor.event-dev"]}' "http://event.monitor.ifengidc.com/event/output"

#### 3 发送报警 （兼容kapacitor）
---

    curl -X POST -d '{"id":"cpu.idle:nil","message":"cpu.idle:nil is CRITICAL","time":"2017-10-27T04:01:55Z","duration":9223372036854775807,"level":"CRITICAL","data":{"Series":[{"name":"cpu.idle","columns":["time","mean"],"values":[["2017-10-27T04:01:55Z",97.17833333333333]]}],"Messages":null,"Err":null}}' "http://127.0.0.1:8090/event/post?version=test.puppet.op.loda__disk.io.util__3f67570e-d1eb-4a91-bad5-1748c47c0335__1d57d04c6e2f407cce02bb28bcd9c0f4" "http://127.0.0.1:8090/event/post?verion=leaf.test.loda__cpu.idle__ef8354e4-66c0-437c-ad4b-b69b6dbc59f7__83673d5330a2300c5aed83444d2776c0"
