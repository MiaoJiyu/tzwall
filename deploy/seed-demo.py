#!/usr/bin/env python3
"""写入演示数据：学生账号、投稿、评论、点赞、举报与一条禁言处罚。

用法：python3 deploy/seed-demo.py [http://127.0.0.1/api]
"""
import json
import sys
import urllib.error
import urllib.request
from datetime import datetime, timedelta, timezone

BASE = (sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1") + "/api"


def unwrap(r):
    """统一响应体 {code,message,data} -> data"""
    if isinstance(r, dict) and "data" in r:
        return r.get("data") or {}
    return r


def req(method, path, body=None, token=None):
    data = json.dumps(body, ensure_ascii=False).encode() if body is not None else None
    r = urllib.request.Request(BASE + path, data=data, method=method)
    r.add_header("Content-Type", "application/json")
    if token:
        r.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(r, timeout=15) as resp:
            return json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        return {"_error": e.code, "_body": e.read().decode()[:300]}
    except Exception as e:  # noqa: BLE001
        return {"_error": -1, "_body": str(e)}


def main():
    # 1) 学生账号
    for i in (1, 2, 3):
        r = req("POST", "/auth/register", {
            "username": f"student0{i}",
            "password": "Student@123",
            "email": f"student0{i}@tzwall.local",
            "real_name": f"同学{i}",
            "school": "泰兴中学",
            "wechat": f"wx_student0{i}",
            "agree": True,
        })
        print(f"[注册] student0{i} ->", "OK" if not r.get("_error") else r)

    # 2) 登录学生 1
    login = unwrap(req("POST", "/auth/login", {"identifier": "student01", "password": "Student@123"}))
    token = login.get("token") if isinstance(login, dict) else None
    if not token:
        print("学生登录失败：", login)
        return
    print("[登录] student01 OK")

    # 3) 取标签
    tags = unwrap(req("GET", "/tags")).get("items", [])
    tag_ids = [t["id"] for t in tags[:2]]
    print(f"[标签] 可用 {len(tags)} 个，使用前 {len(tag_ids)} 个")

    # 4) 投稿
    subs = [
        {"title": "高二（3）班的元旦晚会节目单出来啦",
         "content": "今年元旦晚会有合唱、相声、街舞和校园剧四类节目，各班节目单已贴在公告栏，欢迎同学们积极报名观众互动环节。",
         "anonymous": False, "contact": "wx_student01", "contact_visible": "login"},
        {"title": "图书馆三楼自习室座位太紧张了",
         "content": "期末临近，三楼自习室经常满座。建议同学们错峰前往，也呼吁大家离开时带走个人物品，不要长时间占座。",
         "anonymous": False},
        {"title": "失物招领：操场捡到一串钥匙",
         "content": "昨天下午在操场东侧捡到一串钥匙，带一个蓝色挂饰。请失主到门卫处认领，或在本帖下方留言。",
         "anonymous": True, "contact": "门卫室", "contact_visible": "all"},
    ]
    ids = []
    for s in subs:
        body = dict(s)
        body["tag_ids"] = tag_ids
        r = unwrap(req("POST", "/submissions", body, token))
        if r.get("_error"):
            print("[投稿失败]", r)
        elif r.get("id"):
            ids.append(r.get("id"))
    print("[投稿] 已创建", ids)

    # 管理员登录并审核通过演示投稿（未配置内容审核时投稿默认转人工）
    adm = unwrap(req("POST", "/auth/login", {"identifier": "admin", "password": "Admin@123456"}))
    at = adm.get("token") if isinstance(adm, dict) else None
    if at and ids:
        for sid in ids:
            r = req("POST", f"/admin/submissions/{sid}/review", {"status": "approved", "note": ""}, at)
            print(f"[审核] 投稿 {sid} ->", "OK" if not r.get("_error") else r)

    if ids:
        # 5) 评论与回复
        c = unwrap(req("POST", f"/submissions/{ids[0]}/comments", {"content": "节目单太棒了，期待合唱！", "parent_id": 0}, token))
        if c.get("id"):
            req("POST", f"/submissions/{ids[0]}/comments", {"content": "同期待，记得提前占前排～", "parent_id": c.get("id")}, token)
        req("POST", f"/submissions/{ids[1]}/comments", {"content": "建议开放空教室作为临时自习区。", "parent_id": 0}, token)
        print("[评论] 已创建")

        # 6) 点赞
        req("POST", "/reactions", {"target_type": "submission", "target_id": ids[0], "attitude": 1}, token)
        req("POST", "/reactions", {"target_type": "submission", "target_id": ids[2], "attitude": 1}, token)
        print("[点赞] 已完成")

        # 7) 学生 2 举报第一条投稿
        l2 = unwrap(req("POST", "/auth/login", {"identifier": "student02", "password": "Student@123"}))
        t2 = l2.get("token") if isinstance(l2, dict) else None
        if t2:
            req("POST", "/reports", {"target_type": "submission", "target_id": ids[0], "reason": "标题含不明来源信息，请核实"}, t2)
            print("[举报] student02 已举报投稿 #%s" % ids[0])

    # 8) 管理员：给 student03 一个 7 天禁言（演示处罚效果）
    if not at:
        adm = unwrap(req("POST", "/auth/login", {"identifier": "admin", "password": "Admin@123456"}))
        at = adm.get("token") if isinstance(adm, dict) else None
    if at:
        users = unwrap(req("GET", "/admin/users?keyword=student03", None, at))
        items = users.get("items", []) if isinstance(users, dict) else []
        if items:
            uid = items[0]["id"]
            exp = (datetime.now(timezone.utc) + timedelta(days=7)).isoformat()
            r = req("POST", "/admin/punishments", {
                "user_id": uid, "type": "mute", "reason": "演示：重复发布广告内容",
                "permanent": False, "expires_at": exp,
            }, at)
            print("[处罚] student03 禁言 7 天 ->", "OK" if not r.get("_error") else r)
    else:
        print("管理员登录失败：", adm)

    print("\n演示账号：")
    print("  超级管理员：admin / Admin@123456")
    print("  学生账号：student01 student02 student03 / Student@123")
    print("  说明：student03 已被禁言 7 天，登录后可见禁言横幅且无法投稿/评论/点赞/举报")

    # 9) 阶段三演示数据：示例词表（违禁词用于演示拦截效果）
    print("\n[阶段三] 写入示例词表")
    demo_words = ["测试违禁词", "代写作业", "考试答案"]
    for w in demo_words:
        req("POST", "/admin/words", {"words": w, "category": "forbidden"}, at)
    print("  违禁词示例：", ", ".join(demo_words))
    print("  AI 审核默认关闭，可在后台「系统配置 → 审核与 AI」启用并填入接口信息")


if __name__ == "__main__":
    main()
