#!/bin/bash
# ============================================================================
# 安装上海电信 IPTV 专网路由
#
# 背景：IPTV 专网在独立网卡（本例 eth1，DHCP 下发 30.x.x.x）。系统里存在两条默认路由，
#       公网默认路由（eth0，metric 0）优先，导致去专网服务器（222.68.208.73 认证、
#       218.83.x.x EPG）的流量被送到 LAN 网关而失败。
#       本脚本把这些目的地显式指向专网网卡的网关。
#
# 特性：
#   - 幂等：可重复执行（ip route replace），已被 systemd timer 周期调用
#   - 自动取网关：优先取该网卡上 DHCP 下发的默认路由网关，其次按同网段推测 .1
#   - 可通过 /etc/default/iptv-routes 覆盖网卡与目的地列表
#
# 安装：见同目录 iptv-routes.service / iptv-routes.timer
# ============================================================================
set -u

IFACE="${IPTV_IFACE:-eth1}"
DESTS="${IPTV_ROUTES:-222.68.208.0/24 218.83.0.0/16 124.75.0.0/16 10.192.0.0/16 180.168.0.0/16}"
GW="${IPTV_GW:-}"

log() { echo "[iptv-routes] $*"; }

if [ ! -d "/sys/class/net/$IFACE" ]; then
    log "网卡 $IFACE 不存在，跳过"
    exit 0
fi

# 1) 优先使用该网卡上 DHCP 下发的默认路由网关
if [ -z "$GW" ]; then
    GW="$(ip -4 route show default dev "$IFACE" 2>/dev/null | awk '/via/ {print $3; exit}')"
fi

# 2) 退路：用该网卡同网段的 .1
if [ -z "$GW" ]; then
    GW="$(ip -4 -o addr show dev "$IFACE" 2>/dev/null | awk '{print $4}' | cut -d/ -f1 | head -n1 \
          | awk -F. 'NF==4 {printf "%s.%s.%s.1", $1, $2, $3}')"
    [ -n "$GW" ] && log "未找到 DHCP 默认路由，按同网段推测网关: $GW"
fi

if [ -z "$GW" ]; then
    log "无法确定 $IFACE 的网关（是否已通过 DHCP 获取地址？），退出"
    exit 1
fi

changed=0
for net in $DESTS; do
    if ip route show "$net" 2>/dev/null | grep -q "via $GW dev $IFACE"; then
        continue
    fi
    if ip route replace "$net" via "$GW" dev "$IFACE"; then
        log "已安装 $net via $GW dev $IFACE"
        changed=1
    else
        log "安装失败 $net via $GW dev $IFACE"
    fi
done

if [ "$changed" = "0" ]; then
    log "所有专网路由均已就绪（网关 $GW dev $IFACE）"
fi

exit 0
