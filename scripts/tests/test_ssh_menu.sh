#!/bin/sh
# 3m-ui SSH 管理菜单 —— 修复回归验证
#
# 用途：证明 PR #100 (fix/ssh-menu-config-safety) 修掉的每个缺陷。
# 每个用例都在临时目录里跑「修复前」和「修复后」两份实现并对比结果。
#
# 用法：
#   sh 3m-ui-ssh-menu-regression.sh
#
# 不需要安装 3m-ui；不会读写 /etc/3m-ui、/var/lib/3m-ui。
# 唯一会触碰系统路径的是用例 4，且仅当 /usr/local/bin/3m-ui 不存在时。

set -u

WORK=$(mktemp -d)
PASS=0
FAIL=0
SKIP=0

cleanup() { rm -rf "$WORK"; }
trap cleanup EXIT INT TERM

ok()   { PASS=$((PASS + 1)); printf '  \033[32mPASS\033[0m  %s\n' "$1"; }
bad()  { FAIL=$((FAIL + 1)); printf '  \033[31mFAIL\033[0m  %s\n' "$1"; }
skip() { SKIP=$((SKIP + 1)); printf '  \033[33mSKIP\033[0m  %s\n' "$1"; }

# ---------------------------------------------------------------------------
# 两份 yaml_set 实现
#   yaml_set_old  = main 分支上的原始实现（PR #100 之前）
#   yaml_set_new  = 修复后的实现
# 两者用法相同：yaml_set_* <config_file> <key> <value>
# ---------------------------------------------------------------------------

yaml_set_old() {
    # 原始实现：全局 sed 替换 + mv 覆盖
    _f="$1"; _k="$2"; _v="$3"
    _t="${_f}.tmp.$$"
    if grep -q "^${_k}" "$_f" 2>/dev/null; then
        _e=$(printf '%s' "$_v" | sed -e 's/[\\|&]/\\&/g')
        sed "s|^${_k}.*|${_k} ${_e}|" "$_f" > "$_t" && mv "$_t" "$_f"
    else
        awk -v k="$_k" -v v="$_v" '
            BEGIN { done=0 }
            { print }
            /^server:/ && !done { print k " " v; done=1 }
        ' "$_f" > "$_t" && mv "$_t" "$_f"
    fi
}

yaml_set_new() {
    # 修复实现：作用域限定 server: 块 + 缩进无关 + 写回原 inode
    # 注意：键不带缩进前缀（"port:" 而非 "  port:"），与修复后的脚本一致
    _f="$1"; _k="$2"; _v="$3"
    _t=$(mktemp "${_f}.tmp.XXXXXX") || return 1
    if awk -v k="$_k" '
        /^[^[:space:]#]/ { inblk = (/^server:/) }
        inblk && $1 == k { found = 1 }
        END { exit found ? 0 : 1 }' "$_f"
    then
        awk -v k="$_k" -v v="$_v" '
            /^[^[:space:]#]/ { inblk = (/^server:/) }
            inblk && $1 == k && !done {
                match($0, /^[[:space:]]*/)
                print substr($0, 1, RLENGTH) k " " v
                done = 1
                next
            }
            { print }' "$_f" > "$_t"
    else
        if ! awk -v k="$_k" -v v="$_v" '
            /^[^[:space:]#]/ {
                if (inblk && !ins) { print (ind == "" ? "  " : ind) k " " v; ins = 1 }
                inblk = (/^server:/)
            }
            inblk && /^[[:space:]]+[^[:space:]]/ && ind == "" {
                match($0, /^[[:space:]]*/)
                ind = substr($0, 1, RLENGTH)
            }
            { print }
            END {
                if (inblk && !ins) { print (ind == "" ? "  " : ind) k " " v; ins = 1 }
                else if (!ins) exit 3
            }' "$_f" > "$_t"
        then
            rm -f "$_t"
            return 1
        fi
    fi
    cat "$_t" > "$_f" || { rm -f "$_t"; return 1; }
    rm -f "$_t"
}

# 构造一份「面板 + 节点」混排的配置，节点里也有两空格缩进的 port:
fixture() {
    cat > "$1" <<'EOF'
server:
  port: 8080
  listen: ""
listeners:
  - name: node1
  port: 9000
EOF
    chmod 600 "$1"
}

printf '\n3m-ui SSH 菜单修复回归验证\n'
printf '================================================================\n\n'

# --- 用例 1：改面板端口不应改动节点端口 ------------------------------------
printf '用例 1  改面板端口时，节点端口必须保持不变\n'
d="$WORK/c1"; mkdir -p "$d"

fixture "$d/old.yaml"; yaml_set_old "$d/old.yaml" '  port:' '9999'
old_node=$(awk '/^  port: 9000/{print "9000"}' "$d/old.yaml")
old_hit=$(grep -c 'port: 9999' "$d/old.yaml")

fixture "$d/new.yaml"; yaml_set_new "$d/new.yaml" 'port:' '9999'
new_node=$(grep -c '^  port: 9000' "$d/new.yaml")
new_hit=$(grep -c 'port: 9999' "$d/new.yaml")

printf '    修复前: 被改写的 port 行数 = %s（期望 1）\n' "$old_hit"
printf '    修复后: 被改写的 port 行数 = %s（期望 1）\n' "$new_hit"
if [ "$old_hit" -gt 1 ]; then
    ok   "修复前确实误伤了节点端口（缺陷复现成功）"
else
    bad  "未能复现原缺陷，用例无效"
fi
if [ "$new_hit" -eq 1 ] && [ "$new_node" -eq 1 ]; then
    ok   "修复后节点端口 9000 保持不变，仅面板端口变更"
else
    bad  "修复后仍然误伤或改错：$(cat "$d/new.yaml" | tr '\n' ' ')"
fi

# --- 用例 2：config.yaml 权限不得降级 --------------------------------------
printf '\n用例 2  写入配置后，文件权限必须保持 0600\n'
d="$WORK/c2"; mkdir -p "$d"

fixture "$d/old.yaml"; yaml_set_old "$d/old.yaml" '  port:' '9999'
old_mode=$(stat -c '%a' "$d/old.yaml")

fixture "$d/new.yaml"; yaml_set_new "$d/new.yaml" 'port:' '9999'
new_mode=$(stat -c '%a' "$d/new.yaml")

printf '    修复前: 0600 -> %s\n' "$old_mode"
printf '    修复后: 0600 -> %s\n' "$new_mode"
if [ "$old_mode" != "600" ]; then
    ok   "修复前权限确实降级（缺陷复现成功）"
else
    bad  "未能复现权限降级，用例无效"
fi
if [ "$new_mode" = "600" ]; then
    ok   "修复后权限保持 0600"
else
    bad  "修复后权限仍为 $new_mode"
fi

# --- 用例 3：config port 不带参数应给出用法提示 -----------------------------
printf '\n用例 3  3m-ui config port 不带参数\n'
d="$WORK/c3"; mkdir -p "$d"

old_out=$(sh -c 'set -eu
err() { printf "Error: %s\n" "$*" >&2; exit 1; }
new_port="$2"
case "$new_port" in ""|*[!0-9]*) err "Usage: 3m-ui config port <1-65535>";; esac' _ port 2>&1)

new_out=$(sh -c 'set -eu
err() { printf "Error: %s\n" "$*" >&2; exit 1; }
new_port="${2:-}"
case "$new_port" in ""|*[!0-9]*) err "Usage: 3m-ui config port <1-65535>";; esac' _ port 2>&1)

printf '    修复前: %s\n' "$old_out"
printf '    修复后: %s\n' "$new_out"
case "$old_out" in *"parameter not set"*) ok "修复前确实崩溃（缺陷复现成功）" ;; *) bad "未能复现崩溃，用例无效" ;; esac
case "$new_out" in *"Usage: 3m-ui config port"*) ok "修复后正确打印用法提示" ;; *) bad "修复后仍未打印用法提示" ;; esac

# --- 用例 4：入口指向自身时不得无限 exec -----------------------------------
printf '\n用例 4  入口指向自身时，不得无限 exec\n'
ENTRY=/usr/local/bin/3m-ui
if [ -e "$ENTRY" ]; then
    skip "检测到 $ENTRY 已存在，为避免破坏真实安装而跳过"
else
    d="$WORK/c4"; mkdir -p "$d"

    # 修复前：只检测「指向 Go 二进制」，其余一律 exec
    cat > "$d/old.sh" <<'EOS'
#!/bin/sh
set -eu
APP_BIN=/usr/local/lib/3m-ui/3m-ui-bin
if [ -x /usr/local/bin/3m-ui ] && [ "$(readlink -f /usr/local/bin/3m-ui 2>/dev/null || true)" = "$APP_BIN" ]; then
    echo "Invalid installation"; exit 1
fi
exec /usr/local/bin/3m-ui
EOS
    # 修复后：直接 + 间接自引用双重检测
    cat > "$d/new.sh" <<'EOS'
#!/bin/sh
set -eu
APP_BIN=/usr/local/lib/3m-ui/3m-ui-bin
entry=/usr/local/bin/3m-ui
resolved=$(readlink -f "$entry" 2>/dev/null || true)
self=$(readlink -f "$0" 2>/dev/null || true)
if [ -x "$entry" ] && [ "$resolved" = "$APP_BIN" ]; then
    echo "Invalid installation"; exit 1
fi
if [ -n "$resolved" ] && [ "$resolved" = "$self" ]; then
    echo "Invalid installation: resolves to this script"; exit 1
fi
if [ -n "${THREE_M_UI_MENU_REEXEC:-}" ]; then
    echo "Invalid installation: re-exec loop"; exit 1
fi
THREE_M_UI_MENU_REEXEC=1 exec "$entry"
EOS

    run_case() {
        cp "$1" "$ENTRY"; chmod +x "$ENTRY"
        timeout 5 sh "$ENTRY" >"$d/out" 2>&1
        rc=$?
        rm -f "$ENTRY"
        if [ "$rc" -eq 124 ]; then
            printf 'LOOP'
        else
            printf 'EXIT:%s' "$rc"
        fi
    }

    old_rc=$(run_case "$d/old.sh")
    new_rc=$(run_case "$d/new.sh")
    printf '    修复前: %s（124=5 秒内未退出，即死循环）\n' "$old_rc"
    printf '    修复后: %s\n' "$new_rc"
    case "$old_rc" in *LOOP*|*124*) ok "修复前确实死循环（缺陷复现成功）" ;; *) bad "未能复现死循环，用例无效" ;; esac
    case "$new_rc" in *LOOP*|*124*) bad "修复后仍然死循环" ;; *) ok "修复后快速失败并给出提示" ;; esac
fi

# --- 用例 5：键不存在时应插入到 server: 块末尾 ------------------------------
printf '\n用例 5  键不存在时，插入到 server: 块末尾且不误伤其它段\n'
d="$WORK/c5"; mkdir -p "$d"
cat > "$d/new.yaml" <<'EOF'
server:
  port: 8080
  listen: ""
other:
  port: 7777
EOF
yaml_set_new "$d/new.yaml" 'public_url:' '"https://panel.example"'
if grep -q '^  public_url: "https://panel.example"' "$d/new.yaml" \
   && [ "$(grep -c 'port: 7777' "$d/new.yaml")" -eq 1 ]; then
    ok   "public_url 正确插入 server: 块，other.port 未受影响"
else
    bad  "插入位置错误：$(cat "$d/new.yaml" | tr '\n' ' ')"
fi

# --- 用例 6：没有 server: 段时应报错而非静默成功 ----------------------------
printf '\n用例 6  配置中无 server: 段时，不得静默改写其它段，必须报错\n'
d="$WORK/c6"; mkdir -p "$d"
printf 'other:\n  port: 7777\n' > "$d/old.yaml"
printf 'other:\n  port: 7777\n' > "$d/new.yaml"

yaml_set_old "$d/old.yaml" '  port:' '9999'; old_rc=$?
yaml_set_new "$d/new.yaml" 'port:' '9999'; new_rc=$?

printf '    修复前: 退出码 %s，文件内容：%s\n' "$old_rc" "$(tr '\n' ' ' < "$d/old.yaml")"
printf '    修复后: 退出码 %s，文件内容：%s\n' "$new_rc" "$(tr '\n' ' ' < "$d/new.yaml")"
# 修复前的行为比「静默失败」更糟：sed 全局替换命中了 other 段的 port，
# 把 7777 改成了 9999，且退出码为 0 —— 一个不存在的面板端口被"成功修改"。
if [ "$old_rc" -eq 0 ] && grep -q 9999 "$d/old.yaml"; then
    ok   "修复前静默改写了 other 段的 port（缺陷复现成功）"
else
    bad  "未能复现，用例无效"
fi
if [ "$new_rc" -ne 0 ] && ! grep -q 9999 "$d/new.yaml"; then
    ok   "修复后返回非零且文件未被改动"
else
    bad  "修复后仍未正确报错"
fi

# --- 用例 7：语法检查 -------------------------------------------------------
printf '\n用例 7  sh -n 语法检查（需网络，从分支拉取修复后的脚本）\n'
d="$WORK/c7"; mkdir -p "$d"
if command -v curl >/dev/null 2>&1; then
    curl -sS --max-time 45 -o "$d/3m-ui" \
      "https://raw.githubusercontent.com/kazeyukiro/3m-ui/fix/ssh-menu-config-safety/scripts/3m-ui" 2>/dev/null
    curl -sS --max-time 45 -o "$d/3m-ui.sh" \
      "https://raw.githubusercontent.com/kazeyukiro/3m-ui/fix/ssh-menu-config-safety/scripts/3m-ui.sh" 2>/dev/null
    if [ -s "$d/3m-ui" ] && [ -s "$d/3m-ui.sh" ]; then
        if sh -n "$d/3m-ui" && sh -n "$d/3m-ui.sh"; then
            ok "分支上的两个脚本均通过 sh -n"
        else
            bad "语法检查未通过"
        fi
    else
        skip "下载失败，跳过"
    fi
else
    skip "未找到 curl，跳过"
fi

# --- 用例 8：缩进无关（Go 重写配置后会变成 4 空格） -------------------------
printf '\n用例 8  4 空格缩进配置下改端口，不得产生重复 port 键\n'
d="$WORK/c8"; mkdir -p "$d"
# 这正是 reset-config（SSH 救援）执行后 config.yaml 的真实形态
cat > "$d/old.yaml" <<'EOF'
server:
    mode: release
    port: 8080
    sub_port: 0
database:
    path: /x/3m-ui.db
EOF
cp "$d/old.yaml" "$d/new.yaml"

yaml_set_old "$d/old.yaml" '  port:' '9090'
old_n=$(grep -c '^[[:space:]]*port:' "$d/old.yaml")
yaml_set_new "$d/new.yaml" 'port:' '9090'
new_n=$(grep -c '^[[:space:]]*port:' "$d/new.yaml")

printf '    修复前: 改完后 port 键数量 = %s（期望 1）\n' "$old_n"
printf '    修复后: 改完后 port 键数量 = %s（期望 1）\n' "$new_n"
if [ "$old_n" -gt 1 ]; then
    ok   "修复前产生了重复 port 键（缺陷复现成功）"
else
    bad  "未能复现重复键，用例无效"
fi
if [ "$new_n" -eq 1 ] && grep -q '^    port: 9090' "$d/new.yaml"; then
    ok   "修复后就地改写，缩进与原配置一致"
else
    bad  "修复后仍异常：$(tr '\n' ' ' < "$d/new.yaml")"
fi

# --- 用例 9：改端口后 server 块缩进必须一致 ----------------------------------
printf '\n用例 9  改端口后 server: 块内缩进必须一致（否则 YAML 解析失败）\n'
d="$WORK/c9"; mkdir -p "$d"
# server 块内缩进长度的种类数：>1 即为混合缩进，Go 会报
# "yaml: did not find expected key" 并拒绝启动
mixed() {
    awk '/^server:/ {inblk=1; next} /^[^[:space:]]/ {inblk=0} inblk && /^[[:space:]]+[^[:space:]]/ {
        match($0,/^[[:space:]]*/); print RLENGTH }' "$1" | sort -u | wc -l
}
printf 'server:\n    mode: release\n    port: 8080\n' > "$d/old.yaml"
cp "$d/old.yaml" "$d/new.yaml"

# 修复前：硬编码输出两空格
awk '/^[[:space:]]+port:/ { print "  port: 9090"; next } { print }' "$d/old.yaml" > "$d/old.out"
# 修复后：沿用原行缩进
awk '/^[[:space:]]+port:/ { match($0,/^[[:space:]]*/); print substr($0,1,RLENGTH) "port: 9090"; next } { print }' "$d/new.yaml" > "$d/new.out"

old_kinds=$(mixed "$d/old.out")
new_kinds=$(mixed "$d/new.out")
printf '    修复前: server 块内缩进种类数 = %s（>1 即混合缩进）\n' "$old_kinds"
printf '    修复后: server 块内缩进种类数 = %s\n' "$new_kinds"
if [ "$old_kinds" -gt 1 ]; then
    ok   "修复前确实产生混合缩进（缺陷复现成功）"
else
    bad  "未能复现混合缩进，用例无效"
fi
if [ "$new_kinds" -eq 1 ]; then
    ok   "修复后缩进保持一致"
else
    bad  "修复后仍为混合缩进"
fi

# --- 汇总 -------------------------------------------------------------------
printf '\n================================================================\n'
printf '结果:  \033[32m%s 通过\033[0m   \033[31m%s 失败\033[0m   \033[33m%s 跳过\033[0m\n' "$PASS" "$FAIL" "$SKIP"
printf '================================================================\n\n'
[ "$FAIL" -eq 0 ] || exit 1
