### 1 状态清除接口
---

清楚一个节点下，一个监控项/机器的报警状态及block状态。

    # 清楚一个监控项的报警状态
    curl "http://127.0.0.1/event/clear/status?ns=server.product.loda&alarm=alarm-version"
    # 清除一个机器的报警状态
    curl "http://127.0.0.1/event/clear/status?ns=server.product.loda&host=hostname"