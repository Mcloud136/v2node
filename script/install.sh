#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)

if [ "$(id -u)" -ne 0 ]; then
    echo -e "${red}错误：${plain} 必须使用root用户运行此脚本！"
    exit 1
fi

release=""

if [ -f /etc/redhat-release ]; then
    release="centos"
elif cat /etc/issue 2>/dev/null | grep -Eqi "alpine"; then
    release="alpine"
elif cat /etc/issue 2>/dev/null | grep -Eqi "debian"; then
    release="debian"
elif cat /etc/issue 2>/dev/null | grep -Eqi "ubuntu"; then
    release="ubuntu"
elif cat /etc/issue 2>/dev/null | grep -Eqi "centos|red hat|redhat|rocky|alma|oracle linux"; then
    release="centos"
elif cat /proc/version 2>/dev/null | grep -Eqi "debian"; then
    release="debian"
elif cat /proc/version 2>/dev/null | grep -Eqi "ubuntu"; then
    release="ubuntu"
elif cat /proc/version 2>/dev/null | grep -Eqi "centos|red hat|redhat|rocky|alma|oracle linux"; then
    release="centos"
elif cat /proc/version 2>/dev/null | grep -Eqi "arch"; then
    release="arch"
else
    echo -e "${red}未检测到系统版本，请联系脚本作者！${plain}"
    exit 1
fi

VERSION_ARG=""
API_HOST_ARG=""
NODE_ID_ARG=""
API_KEY_ARG=""

parse_args() {
    while [ $# -gt 0 ]; do
        case "$1" in
            --api-host)
                API_HOST_ARG="$2"; shift 2 ;;
            --node-id)
                NODE_ID_ARG="$2"; shift 2 ;;
            --api-key)
                API_KEY_ARG="$2"; shift 2 ;;
            -h|--help)
                echo "用法: $0 [版本号] [--api-host URL] [--node-id ID] [--api-key KEY]"
                exit 0 ;;
            --*)
                echo "未知参数: $1"; exit 1 ;;
            *)
                if [ -z "$VERSION_ARG" ]; then
                    VERSION_ARG="$1"; shift
                else
                    shift
                fi ;;
        esac
    done
}

arch=$(uname -m)

if [ "$arch" = "x86_64" ] || [ "$arch" = "x64" ] || [ "$arch" = "amd64" ]; then
    arch="64"
elif [ "$arch" = "aarch64" ] || [ "$arch" = "arm64" ]; then
    arch="arm64-v8a"
elif [ "$arch" = "s390x" ]; then
    arch="s390x"
else
    arch="64"
    echo -e "${red}检测架构失败，使用默认架构: ${arch}${plain}"
fi

if [ "$(getconf WORD_BIT)" != '32' ] && [ "$(getconf LONG_BIT)" != '64' ]; then
    echo "本软件不支持 32 位系统(x86)，请使用 64 位系统(x86_64)，如果检测有误，请联系作者"
    exit 2
fi

os_version=""
if [ -f /etc/os-release ]; then
    os_version=$(awk -F'[= ."]' '/VERSION_ID/{print $3}' /etc/os-release)
fi
if [ -z "$os_version" ] && [ -f /etc/lsb-release ]; then
    os_version=$(awk -F'[= ."]+' '/DISTRIB_RELEASE/{print $2}' /etc/lsb-release)
fi

if [ "x${release}" = "xcentos" ]; then
    if [ -n "$os_version" ] && [ "$os_version" -le 6 ]; then
        echo -e "${red}请使用 CentOS 7 或更高版本的系统！${plain}"
        exit 1
    fi
    if [ -n "$os_version" ] && [ "$os_version" -eq 7 ]; then
        echo -e "${red}注意： CentOS 7 无法使用hysteria1/2协议！${plain}"
    fi
elif [ "x${release}" = "xubuntu" ]; then
    if [ -n "$os_version" ] && [ "$os_version" -lt 16 ]; then
        echo -e "${red}请使用 Ubuntu 16 或更高版本的系统！${plain}"
        exit 1
    fi
elif [ "x${release}" = "debian" ]; then
    if [ -n "$os_version" ] && [ "$os_version" -lt 8 ]; then
        echo -e "${red}请使用 Debian 8 或更高版本的系统！${plain}"
        exit 1
    fi
fi

install_base() {
    need_install_apt() {
        local packages="$*"
        local missing=""
        
        local installed_list=$(dpkg-query -W -f='${Package}\n' 2>/dev/null | sort)
        
        for p in $packages; do
            if ! echo "$installed_list" | grep -q "^${p}$"; then
                missing="$missing $p"
            fi
        done
        
        if [ -n "$missing" ]; then
            echo "安装缺失的包: $missing"
            apt-get update -y >/dev/null 2>&1
            DEBIAN_FRONTEND=noninteractive apt-get install -y $missing >/dev/null 2>&1
        fi
    }

    need_install_yum() {
        local packages="$*"
        local missing=""
        
        local installed_list=$(rpm -qa --qf '%{NAME}\n' 2>/dev/null | sort)
        
        for p in $packages; do
            if ! echo "$installed_list" | grep -q "^${p}$"; then
                missing="$missing $p"
            fi
        done
        
        if [ -n "$missing" ]; then
            echo "安装缺失的包: $missing"
            yum install -y $missing >/dev/null 2>&1
        fi
    }

    need_install_apk() {
        local packages="$*"
        local missing=""
        
        local installed_list=$(apk info 2>/dev/null | sort)
        
        for p in $packages; do
            if ! echo "$installed_list" | grep -q "^${p}$"; then
                missing="$missing $p"
            fi
        done
        
        if [ -n "$missing" ]; then
            echo "安装缺失的包: $missing"
            apk add --no-cache $missing >/dev/null 2>&1
        fi
    }

    if [ "x${release}" = "xcentos" ]; then
        if ! rpm -q epel-release >/dev/null 2>&1; then
            echo "安装 EPEL 源..."
            yum install -y epel-release >/dev/null 2>&1
        fi
        need_install_yum wget curl unzip tar cronie socat ca-certificates pv
        update-ca-trust force-enable >/dev/null 2>&1 || true
    elif [ "x${release}" = "xalpine" ]; then
        need_install_apk wget curl unzip tar socat ca-certificates pv
        update-ca-certificates >/dev/null 2>&1 || true
    elif [ "x${release}" = "xdebian" ]; then
        need_install_apt wget curl unzip tar cron socat ca-certificates pv
        update-ca-certificates >/dev/null 2>&1 || true
    elif [ "x${release}" = "xubuntu" ]; then
        need_install_apt wget curl unzip tar cron socat ca-certificates pv
        update-ca-certificates >/dev/null 2>&1 || true
    elif [ "x${release}" = "xarch" ]; then
        echo "更新包数据库..."
        pacman -Sy --noconfirm >/dev/null 2>&1
        echo "安装必需的包..."
        pacman -S --noconfirm --needed wget curl unzip tar cronie socat ca-certificates pv >/dev/null 2>&1
    fi
}

check_status() {
    if [ ! -f /usr/local/v2node/v2node ]; then
        return 2
    fi
    if [ "x${release}" = "xalpine" ]; then
        temp=$(service v2node status 2>/dev/null | awk '{print $3}')
        if [ "x${temp}" = "xstarted" ]; then
            return 0
        else
            return 1
        fi
    else
        temp=$(systemctl status v2node 2>/dev/null | grep Active | awk '{print $3}' | cut -d "(" -f2 | cut -d ")" -f1)
        if [ "x${temp}" = "xrunning" ]; then
            return 0
        else
            return 1
        fi
    fi
}

generate_v2node_config() {
    local api_host="$1"
    local node_id="$2"
    local api_key="$3"

    mkdir -p /etc/v2node >/dev/null 2>&1
    cat > /etc/v2node/config.json <<EOF
{
    "Log": {
        "Level": "warning",
        "Output": "",
        "Access": "none"
    },
    "Nodes": [
        {
            "ApiHost": "${api_host}",
            "NodeID": ${node_id},
            "ApiKey": "${api_key}",
            "Timeout": 15
        }
    ]
}
EOF
    echo -e "${green}V2node 配置文件生成完成,正在重新启动服务${plain}"
    if [ "x${release}" = "xalpine" ]; then
        service v2node restart 2>/dev/null || true
        service cloudflared restart 2>/dev/null || true
    else
        systemctl restart v2node 2>/dev/null || true
        systemctl restart cloudflared.service 2>/dev/null || true
    fi
    sleep 2
    check_status
    echo ""
    if [ $? -eq 0 ]; then
        echo -e "${green}v2node 重启成功${plain}"
    else
        echo -e "${red}v2node 可能启动失败，请使用 v2node log 查看日志信息${plain}"
    fi
}

install_v2node() {
    local version_param="$1"
    if [ -e /usr/local/v2node/ ]; then
        rm -rf /usr/local/v2node/
    fi

    mkdir -p /usr/local/v2node/
    cd /usr/local/v2node/ || exit 1

    if [ -z "$version_param" ]; then
        last_version=$(curl -Ls "https://api.github.com/repos/Mcloud136/v2node/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [ -z "$last_version" ]; then
            echo -e "${red}检测 v2node 版本失败，可能是超出 Github API 限制，请稍后再试，或手动指定 v2node 版本安装${plain}"
            exit 1
        fi
        echo -e "${green}检测到最新版本：${last_version}，开始安装...${plain}"
        url="https://github.com/Mcloud136/v2node/releases/download/${last_version}/v2node-linux-${arch}.zip"
        curl -sL "$url" | pv -s 30M -W -N "下载进度" > /usr/local/v2node/v2node-linux.zip
        if [ $? -ne 0 ]; then
            echo -e "${red}下载 v2node 失败，请确保你的服务器能够下载 Github 的文件${plain}"
            exit 1
        fi
    else
        last_version="$version_param"
        url="https://github.com/Mcloud136/v2node/releases/download/${last_version}/v2node-linux-${arch}.zip"
        curl -sL "$url" | pv -s 30M -W -N "下载进度" > /usr/local/v2node/v2node-linux.zip
        if [ $? -ne 0 ]; then
            echo -e "${red}下载 v2node $1 失败，请确保此版本存在${plain}"
            exit 1
        fi
    fi

    unzip v2node-linux.zip 2>/dev/null || true
    rm -f v2node-linux.zip
    chmod +x v2node
    mkdir -p /etc/v2node/
    cp geoip.dat /etc/v2node/ 2>/dev/null || true
    cp geosite.dat /etc/v2node/ 2>/dev/null || true
    
    first_install="false"
    
    if [ "x${release}" = "xalpine" ]; then
        rm -f /etc/init.d/v2node
        cat > /etc/init.d/v2node <<EOF
#!/sbin/openrc-run

name="v2node"
description="v2node"

command="/usr/local/v2node/v2node"
command_args="server"
command_user="root"

pidfile="/run/v2node.pid"
command_background="yes"

start_post() {
        service cloudflared restart 2>/dev/null || true
}

depend() {
        need net
}
EOF
        chmod +x /etc/init.d/v2node
        rc-update add v2node default 2>/dev/null || true
        echo -e "${green}v2node ${last_version}${plain} 安装完成，已设置开机自启"
    else
        rm -f /etc/systemd/system/v2node.service
        cat > /etc/systemd/system/v2node.service <<EOF
[Unit]
Description=v2node Service
After=network.target nss-lookup.target
Wants=network.target

[Service]
User=root
Group=root
Type=simple
LimitAS=infinity
LimitRSS=infinity
LimitCORE=infinity
LimitNOFILE=999999
WorkingDirectory=/usr/local/v2node/
ExecStart=/usr/local/v2node/v2node server
ExecStartPost=/bin/sh -c '/bin/systemctl restart cloudflared.service 2>/dev/null || true'
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF
        systemctl daemon-reload 2>/dev/null || true
        systemctl stop v2node 2>/dev/null || true
        systemctl enable v2node 2>/dev/null || true
        echo -e "${green}v2node ${last_version}${plain} 安装完成，已设置开机自启"
    fi

    if [ ! -f /etc/v2node/config.json ]; then
        if [ -n "$API_HOST_ARG" ] && [ -n "$NODE_ID_ARG" ] && [ -n "$API_KEY_ARG" ]; then
            generate_v2node_config "$API_HOST_ARG" "$NODE_ID_ARG" "$API_KEY_ARG"
            echo -e "${green}已根据参数生成 /etc/v2node/config.json${plain}"
            first_install="false"
        else
            cp config.json /etc/v2node/ 2>/dev/null || true
            first_install="true"
        fi
    else
        if [ "x${release}" = "xalpine" ]; then
            service v2node start 2>/dev/null || true
        else
            systemctl start v2node 2>/dev/null || true
        fi
        sleep 2
        check_status
        echo ""
        if [ $? -eq 0 ]; then
            echo -e "${green}v2node 重启成功${plain}"
        else
            echo -e "${red}v2node 可能启动失败，请使用 v2node log 查看日志信息${plain}"
        fi
        first_install="false"
    fi


    curl -o /usr/bin/v2node -Ls https://raw.githubusercontent.com/Mcloud136/v2node/main/script/v2node.sh
    chmod +x /usr/bin/v2node

    cd "$cur_dir" || exit 1
    rm -f install.sh
    echo "------------------------------------------"
    echo -e "管理脚本使用方法: "
    echo "------------------------------------------"
    echo "v2node              - 显示管理菜单 (功能更多)"
    echo "v2node start        - 启动 v2node"
    echo "v2node stop         - 停止 v2node"
    echo "v2node restart      - 重启 v2node"
    echo "v2node status       - 查看 v2node 状态"
    echo "v2node enable       - 设置 v2node 开机自启"
    echo "v2node disable      - 取消 v2node 开机自启"
    echo "v2node log          - 查看 v2node 日志"
    echo "v2node generate     - 生成 v2node 配置文件"
    echo "v2node update       - 更新 v2node"
    echo "v2node update x.x.x - 更新 v2node 指定版本"
    echo "v2node install      - 安装 v2node"
    echo "v2node uninstall    - 卸载 v2node"
    echo "v2node version      - 查看 v2node 版本"
    echo "------------------------------------------"
    curl -fsS --max-time 10 "https://api.v-50.me/counter" 2>/dev/null || true

    if [ "$first_install" = "true" ]; then
        read -rp "检测到你为第一次安装 v2node，是否自动生成 /etc/v2node/config.json？(y/n): " if_generate
        if echo "$if_generate" | grep -q "^[Yy]$"; then
            read -rp "面板API地址[格式: https://example.com/]: " api_host
            api_host=${api_host:-https://example.com/}
            read -rp "节点ID: " node_id
            node_id=${node_id:-1}
            read -rp "节点通讯密钥: " api_key

            generate_v2node_config "$api_host" "$node_id" "$api_key"
        else
            echo "${green}已跳过自动生成配置。如需后续生成，可执行: v2node generate${plain}"
        fi
    fi
}

parse_args "$@"
echo -e "${green}开始安装${plain}"
install_base
install_v2node "$VERSION_ARG"
