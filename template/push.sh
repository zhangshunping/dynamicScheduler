#!/bin/bash

instance=$(hostname)
ip=$(ifconfig eth0 | grep "inet" |awk '{print $2}'|xargs)
## 配置pushgateway地址
pushgatewayaddr="XXXXXXXXXXXXXXX"
job="kubeNodeMemoryAvailable"



## kubelet 计算kubeletr
getMemory(){
# current memory usage
memory_capacity_in_kb=$(cat /proc/meminfo | grep MemTotal | awk '{print $2}')
memory_capacity_in_bytes=$((memory_capacity_in_kb * 1024))
memory_usage_in_bytes=$(cat /sys/fs/cgroup/memory/memory.usage_in_bytes)
memory_total_inactive_file=$(cat /sys/fs/cgroup/memory/memory.stat | grep total_inactive_file | awk '{print $2}')

memory_working_set=${memory_usage_in_bytes}
if [ "$memory_working_set" -lt "$memory_total_inactive_file" ];
then
    memory_working_set=0
else
    memory_working_set=$((memory_usage_in_bytes - memory_total_inactive_file))
fi

memory_available_in_bytes=$((memory_capacity_in_bytes - memory_working_set))
memory_available_in_kb=$((memory_available_in_bytes / 1024))
memory_available_in_mb=$((memory_available_in_kb / 1024))

echo $memory_available_in_mb



}


pushNodeMem(){

cat << EOF |curl --data-binary @- $pushgatewayaddr/metrics/job/CollectK8sCgroupsMem/instance/$instance
k8_node_cgroups_mem_available{ip="$ip"} $(getMemory)
EOF
}


pushNodeMem
