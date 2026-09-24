#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
HyperDNS — Telegram Sales & Subscription Controller Bot
A comprehensive Telegram bot for selling smart DNS plans, issuing free trials,
managing subscriber accounts, handling payments, and providing real-time telemetry
via the HyperDNS REST API v2.
"""

import html
import json
import logging
import os
import sys
import time
import requests

# -----------------------------------------------------------------------------
# Configuration
# -----------------------------------------------------------------------------
TELEGRAM_BOT_TOKEN = os.getenv("TELEGRAM_BOT_TOKEN", "")
HYPERDNS_API_BASE = os.getenv("HYPERDNS_API_BASE", "http://127.0.0.1:8080/api/v2")
HYPERDNS_API_KEY = os.getenv("HYPERDNS_API_KEY", "")
TELEGRAM_ADMIN_CHAT_IDS = os.getenv("TELEGRAM_ADMIN_CHAT_IDS", "")

# Payment & Sales Customization
PAYMENT_CARD_NUMBER = os.getenv("PAYMENT_CARD_NUMBER", "6037-9975-0000-0000")
PAYMENT_CARD_HOLDER = os.getenv("PAYMENT_CARD_HOLDER", "HyperDNS Sales Team")
PAYMENT_CRYPTO_WALLET = os.getenv("PAYMENT_CRYPTO_WALLET", "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t (USDT TRC20)")
SUPPORT_ADMIN_USERNAME = os.getenv("SUPPORT_ADMIN_USERNAME", "@HyperDNS_Support")

STORE_FILE_PATH = os.path.join(os.path.dirname(__file__), "bot_sales_data.json")

PLANS = [
    {
        "id": "plan_1m",
        "name": "🥉 پلان ۱ ماهه استاندارد (1 Month)",
        "days": 30,
        "traffic_gb": 60.0,
        "max_devices": 2,
        "price_toman": "۱۲۰,۰۰۰ تومان",
        "price_crypto": "2 USDT",
        "desc": "۶۰ گیگابایت ترافیک • حداکثر ۲ دستگاه همزمان • پشتیبانی کامل بازی‌ها"
    },
    {
        "id": "plan_3m",
        "name": "🥈 پلان ۳ ماهه طلایی (3 Months)",
        "days": 90,
        "traffic_gb": 180.0,
        "max_devices": 3,
        "price_toman": "۳۰۰,۰۰۰ تومان",
        "price_crypto": "5 USDT",
        "desc": "۱۸۰ گیگابایت ترافیک • ۳ دستگاه همزمان • تخفیف ویژه دورهٔ ۳ ماهه"
    },
    {
        "id": "plan_vip",
        "name": "🥇 پلان VIP گیمینگ نامحدود (VIP Gaming)",
        "days": 90,
        "traffic_gb": 0.0, # 0 = Unlimited
        "max_devices": 4,
        "price_toman": "۴۸۰,۰۰۰ تومان",
        "price_crypto": "8 USDT",
        "desc": "ترافیک نامحدود • ۴ دستگاه همزمان • پینگ فوق‌العاده پایدار برای کنسول و PC"
    },
    {
        "id": "plan_lifetime",
        "name": "👑 پلان ۱ ساله حرفه‌ای (1 Year Unlimited)",
        "days": 365,
        "traffic_gb": 0.0, # 0 = Unlimited
        "max_devices": 5,
        "price_toman": "۱,۱۰۰,۰۰۰ تومان",
        "price_crypto": "18 USDT",
        "desc": "۱ سال کامل ترافیک نامحدود • ۵ دستگاه همزمان • پشتیبانی اختصاصی VIP"
    }
]

def parse_admin_chat_ids(raw: str) -> set:
    ids = set()
    for part in raw.split(","):
        part = part.strip()
        if part.lstrip("-").isdigit():
            ids.add(int(part))
    return ids

logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
logger = logging.getLogger("HyperDNS-Bot")

def require_env(name: str, value: str) -> str:
    if not value.strip():
        sys.exit(
            f"[FATAL] {name} is not set.\n"
            f"        Set it in the environment, your systemd unit, or a .env file:\n"
            f"            export {name}=...\n"
            f"        HyperDNS deliberately ships no credential defaults in source."
        )
    return value.strip()


# -----------------------------------------------------------------------------
# Local JSON Store for Sales & State
# -----------------------------------------------------------------------------
class SalesStore:
    def __init__(self, path: str):
        self.path = path
        self.data = {
            "trials": {},       # user_id -> timestamp
            "user_clients": {}, # user_id -> client_id
            "orders": {},       # order_id -> dict
            "bot_users": [],    # list of user_ids for broadcast
            "user_states": {}   # user_id -> state string
        }
        self.load()

    def load(self):
        if os.path.exists(self.path):
            try:
                with open(self.path, "r", encoding="utf-8") as f:
                    loaded = json.load(f)
                    for k in self.data:
                        if k in loaded:
                            self.data[k] = loaded[k]
            except Exception as e:
                logger.warning(f"Could not load sales store: {e}")

    def save(self):
        try:
            with open(self.path, "w", encoding="utf-8") as f:
                json.dump(self.data, f, ensure_ascii=False, indent=2)
        except Exception as e:
            logger.error(f"Could not save sales store: {e}")

    def add_user(self, user_id: int):
        if user_id not in self.data["bot_users"]:
            self.data["bot_users"].append(user_id)
            self.save()

    def set_state(self, user_id: int, state: str):
        self.data["user_states"][str(user_id)] = state
        self.save()

    def get_state(self, user_id: int) -> str:
        return self.data["user_states"].get(str(user_id), "")

    def clear_state(self, user_id: int):
        self.data["user_states"].pop(str(user_id), None)
        self.save()


# -----------------------------------------------------------------------------
# HyperDNS REST API Client
# -----------------------------------------------------------------------------
class HyperDNSClient:
    """HTTP client wrapping the HyperDNS REST API v2."""
    def __init__(self, base_url: str, api_key: str):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.session = requests.Session()

    def _headers(self):
        return {
            "X-API-Key": self.api_key,
            "Content-Type": "application/json"
        }

    def get_version(self) -> dict:
        r = self.session.get(f"{self.base_url}/version")
        r.raise_for_status()
        return r.json()

    def get_status(self) -> dict:
        r = self.session.get(f"{self.base_url}/status", headers=self._headers())
        r.raise_for_status()
        return r.json()

    def list_clients(self) -> list:
        r = self.session.get(f"{self.base_url}/clients?limit=500", headers=self._headers())
        r.raise_for_status()
        return r.json().get("items", [])

    def get_client(self, client_id: str) -> dict:
        r = self.session.get(f"{self.base_url}/clients/{client_id}", headers=self._headers())
        r.raise_for_status()
        return r.json()

    def create_client(self, name: str, days: int = 30, ip: str = "", traffic_gb: float = 0.0, policies: list = None, note: str = "", max_devices: int = 2) -> dict:
        payload = {
            "display_name": name,
            "validity_days": days,
            "allowed_ips": [ip] if ip else [],
            "quota_limit_gb": traffic_gb,
            "policy_ids": policies or [],
            "note": note,
            "max_devices": max_devices
        }
        r = self.session.post(f"{self.base_url}/clients", json=payload, headers=self._headers())
        r.raise_for_status()
        return r.json()

    def update_client(self, client_id: str, updates: dict) -> dict:
        r = self.session.patch(f"{self.base_url}/clients/{client_id}", json=updates, headers=self._headers())
        r.raise_for_status()
        return r.json()

    def reset_traffic(self, client_id: str) -> bool:
        r = self.session.post(f"{self.base_url}/clients/{client_id}/actions/reset-traffic", headers=self._headers())
        r.raise_for_status()
        return r.json().get("reset", False)

    def regenerate_uuid(self, client_id: str) -> str:
        r = self.session.post(f"{self.base_url}/clients/{client_id}/actions/regenerate-uuid", headers=self._headers())
        r.raise_for_status()
        return r.json().get("uuid", "")

    def delete_client(self, client_id: str) -> bool:
        r = self.session.delete(f"{self.base_url}/clients/{client_id}", headers=self._headers())
        r.raise_for_status()
        return r.json().get("deleted", False)

    def list_policies(self) -> list:
        r = self.session.get(f"{self.base_url}/policies", headers=self._headers())
        r.raise_for_status()
        return r.json().get("policies", [])

    def toggle_policy(self, key: str, enabled: bool) -> dict:
        r = self.session.post(f"{self.base_url}/policies", json={"key": key, "enabled": enabled}, headers=self._headers())
        r.raise_for_status()
        return r.json()

    def flush_cache(self) -> bool:
        r = self.session.post(f"{self.base_url}/cache/flush", headers=self._headers())
        r.raise_for_status()
        return r.json().get("flushed", False)

    def add_custom_domain(self, client_id: str, domain: str, action: str = "PROXY", include_subs: bool = True) -> dict:
        payload = {
            "domain": domain,
            "action": action,
            "include_subdomains": include_subs
        }
        r = self.session.post(f"{self.base_url}/clients/{client_id}/domains", json=payload, headers=self._headers())
        r.raise_for_status()
        return r.json()


# -----------------------------------------------------------------------------
# Telegram Sales & Management Bot
# -----------------------------------------------------------------------------
class HyperDNSTelegramBot:
    def __init__(self, token: str, api_client: HyperDNSClient, admin_chat_ids: set = None):
        self.token = token
        self.api = api_client
        self.api_url = f"https://api.telegram.org/bot{token}"
        self.last_update_id = 0
        self.admin_chat_ids = admin_chat_ids if admin_chat_ids is not None else set()
        self.store = SalesStore(STORE_FILE_PATH)

    def is_admin(self, chat_id) -> bool:
        return chat_id is not None and chat_id in self.admin_chat_ids

    def send_message(self, chat_id: int, text: str, reply_markup: dict = None) -> dict:
        url = f"{self.api_url}/sendMessage"
        payload = {
            "chat_id": chat_id,
            "text": text,
            "parse_mode": "HTML",
            "disable_web_page_preview": True
        }
        if reply_markup:
            payload["reply_markup"] = json.dumps(reply_markup)
        try:
            r = requests.post(url, json=payload, timeout=10)
            return r.json()
        except Exception as e:
            logger.error(f"Failed to send Telegram message: {e}")
            return {"ok": False, "error": str(e)}

    def answer_callback(self, callback_query_id: str, text: str = ""):
        url = f"{self.api_url}/answerCallbackQuery"
        try:
            requests.post(url, json={"callback_query_id": callback_query_id, "text": text}, timeout=5)
        except Exception as e:
            logger.debug(f"answerCallbackQuery error: {e}")

    # ---------------- Keyboards ----------------

    def main_customer_keyboard(self, is_admin: bool = False) -> dict:
        kb = [
            [
                {"text": "🛍 خرید اشتراک هوشمند", "callback_data": "menu_buy"},
                {"text": "🎁 اکانت تست رایگان", "callback_data": "menu_trial"}
            ],
            [
                {"text": "👤 وضعیت اشتراک من", "callback_data": "menu_mysub"},
                {"text": "🔄 بروزرسانی آی‌پی من", "callback_data": "menu_update_ip"}
            ],
            [
                {"text": "📚 راهنمای تنظیم دستگاه‌ها", "callback_data": "menu_guides"},
                {"text": "📞 ارتباط با پشتیبانی", "callback_data": "menu_support"}
            ]
        ]
        if is_admin:
            kb.append([{"text": "⚙️ پنل مدیریت سرور (Admin)", "callback_data": "menu_admin"}])
        return {"inline_keyboard": kb}

    def admin_keyboard(self) -> dict:
        return {
            "inline_keyboard": [
                [
                    {"text": "📊 وضعیت و تلمتری سرور", "callback_data": "admin_status"},
                    {"text": "👥 لیست مشترکین", "callback_data": "admin_clients"}
                ],
                [
                    {"text": "📥 سفارشات در انتظار تایید", "callback_data": "admin_pending_orders"},
                    {"text": "🧹 پاکسازی کش DNS", "callback_data": "admin_flush"}
                ],
                [
                    {"text": "📢 ارسال پیام همگانی", "callback_data": "admin_broadcast"},
                    {"text": "🔙 بازگشت به منوی اصلی", "callback_data": "menu_main"}
                ]
            ]
        }

    def plans_keyboard(self) -> dict:
        kb = []
        for p in PLANS:
            kb.append([{"text": f"{p['name']} — {p['price_toman']}", "callback_data": f"plan_{p['id']}"}])
        kb.append([{"text": "🔙 بازگشت", "callback_data": "menu_main"}])
        return {"inline_keyboard": kb}

    def plan_payment_keyboard(self, plan_id: str) -> dict:
        return {
            "inline_keyboard": [
                [{"text": "📸 ثبت و ارسال رسید پرداخت", "callback_data": f"pay_receipt_{plan_id}"}],
                [{"text": "🔙 انتخاب پلان دیگر", "callback_data": "menu_buy"}]
            ]
        }

    def guides_keyboard(self) -> dict:
        return {
            "inline_keyboard": [
                [
                    {"text": "🎮 پلی‌استیشن (PS4/PS5)", "callback_data": "guide_ps"},
                    {"text": "🎮 ایکس‌باکس (Xbox)", "callback_data": "guide_xbox"}
                ],
                [
                    {"text": "💻 ویندوز (Windows)", "callback_data": "guide_win"},
                    {"text": "📱 اندروید و آیفون", "callback_data": "guide_mobile"}
                ],
                [
                    {"text": "🌐 مودم و روتر", "callback_data": "guide_router"},
                    {"text": "🔙 بازگشت", "callback_data": "menu_main"}
                ]
            ]
        }

    # ---------------- Formatters ----------------

    def format_client_card(self, c: dict, public_ip: str) -> str:
        esc = html.escape
        policies = c.get("policy_ids") or c.get("custom_policies") or []
        policy_str = esc(", ".join(policies)) if policies else "تمام سرویس‌ها (Global Inherit)"
        status_emoji = "🟢 فعال" if c.get("enabled", True) else "🔴 غیرفعال"
        quota = c.get('quota_limit_gb') or c.get('traffic_limit_gb') or 0
        traffic_limit = f"{quota:.1f} GB" if quota > 0 else "نامحدود"
        traffic_used_mb = c.get('traffic_used_bytes', 0) / (1024 * 1024)

        token = c.get('token', '')
        reg_secret = c.get('register_secret', '')
        portal_link = f"http://{public_ip}:8080/sub/{token}"
        reg_api = f"http://{public_ip}:8080/ip/{token}"

        max_dev = c.get("max_devices", 2)
        allowed_ips = c.get("allowed_ips", [])
        ips_str = esc(", ".join(allowed_ips)) if allowed_ips else "هنوز ثبت نشده (اتصال خودکار)"

        custom_domains = c.get("custom_domains", [])
        domains_count = len(custom_domains)

        return (
            f"⚡ <b>مشخصات اشتراک هوشمند شما:</b>\n"
            f"━━━━━━━━━━━━━━━━━━━\n"
            f"👤 <b>نام کاربر:</b> <code>{esc(str(c.get('display_name') or c.get('name')))}</code>\n"
            f"🔑 <b>شناسه اشتراک:</b> <code>{esc(str(c.get('id')))}</code>\n"
            f"📊 <b>وضعیت:</b> {status_emoji}\n"
            f"📱 <b>دستگاه‌های مجاز:</b> <code>{len(allowed_ips)} از {max_dev} دستگاه</code>\n"
            f"🌐 <b>آی‌پی‌های متصل:</b> <code>{ips_str}</code>\n"
            f"📦 <b>مصرف ترافیک:</b> <code>{traffic_used_mb:.2f} MB / {esc(traffic_limit)}</code>\n"
            f"⏳ <b>اعتبار تا:</b> <code>{esc(str(c.get('expires_at') or 'مادام‌العمر'))}</code>\n"
            f"🎯 <b>سیاست‌های فعال:</b> <i>{policy_str}</i>\n"
            f"🌐 <b>دامنه‌های اختصاصی ذخیره‌شده:</b> <code>{domains_count} دامنه</code>\n\n"
            f"🔐 <b>رمز ثبت اختصاصی (Registration Secret):</b>\n<code>{esc(reg_secret)}</code>\n\n"
            f"🔗 <b>لینک پورتال سابسکریپشن شما:</b>\n<code>{portal_link}</code>\n\n"
            f"📡 <b>تنظیمات سرور DNS:</b>\n"
            f"• Primary DNS: <code>{public_ip}</code>\n"
            f"• Secondary DNS: <code>1.1.1.1</code>\n"
        )

    # ---------------- Processors ----------------

    def process_message(self, message: dict):
        chat_id = message.get("chat", {}).get("id")
        user_id = message.get("from", {}).get("id", chat_id)
        if not chat_id:
            return

        self.store.add_user(chat_id)
        is_admin_user = self.is_admin(chat_id)
        text = message.get("text", "").strip()

        # Handle Photo / Receipt Submission
        state = self.store.get_state(user_id)
        if state.startswith("waiting_receipt:"):
            plan_id = state.split(":", 1)[1]
            plan = next((p for p in PLANS if p["id"] == plan_id), PLANS[0])
            receipt_info = "رسید تصویری ارسال شد" if "photo" in message else (text or "رسید بدون متن")
            order_id = f"ord_{int(time.time())}_{user_id % 1000}"

            self.store.data["orders"][order_id] = {
                "order_id": order_id,
                "user_id": user_id,
                "chat_id": chat_id,
                "username": message.get("from", {}).get("username", "بدون یوزرنیم"),
                "first_name": message.get("from", {}).get("first_name", "کاربر"),
                "plan_id": plan_id,
                "plan_name": plan["name"],
                "price": plan["price_toman"],
                "receipt": receipt_info,
                "status": "pending",
                "created_at": time.time()
            }
            self.store.clear_state(user_id)

            # Confirm to user
            self.send_message(
                chat_id,
                f"✅ <b>رسید شما با کد پیگیری <code>{order_id}</code> با موفقیت ثبت شد!</b>\n\n"
                f"سفارش شما در صف بررسی ادمین قرار گرفت. معمولاً طی ۵ الی ۱۵ دقیقه اشتراک شما فعال شده و در همین ربات برایتان ارسال خواهد شد.",
                self.main_customer_keyboard(is_admin_user)
            )

            # Alert Admins
            admin_msg = (
                f"🔔 <b>سفارش جدید پرداخت دریافت شد!</b>\n"
                f"━━━━━━━━━━━━━━━━━━━\n"
                f"• <b>کد سفارش:</b> <code>{order_id}</code>\n"
                f"• <b>کاربر:</b> {message.get('from', {}).get('first_name', '')} (<code>{user_id}</code>)\n"
                f"• <b>آیدی تلگرام:</b> @{message.get('from', {}).get('username', 'ندارد')}\n"
                f"• <b>پلان انتخابی:</b> {plan['name']}\n"
                f"• <b>مبلغ:</b> {plan['price_toman']}\n"
                f"• <b>شرح رسید:</b> {html.escape(receipt_info)}\n"
            )
            admin_markup = {
                "inline_keyboard": [
                    [
                        {"text": "✅ تایید و صدور اکانت", "callback_data": f"approve_{order_id}"},
                        {"text": "❌ رد سفارش", "callback_data": f"reject_{order_id}"}
                    ]
                ]
            }
            for aid in self.admin_chat_ids:
                self.send_message(aid, admin_msg, admin_markup)
            return

        # Handle /setip or waiting for IP
        if state == "waiting_ip" or text.startswith("/setip"):
            ip_val = text.replace("/setip", "").strip()
            self.store.clear_state(user_id)
            client_id = self.store.data["user_clients"].get(str(user_id))
            if not client_id:
                self.send_message(chat_id, "⚠️ شما هنوز اشتراکی به این حساب متصل نکرده‌اید. ابتدا با دستور /link شناسه اشتراک خود را متصل فرمایید.")
                return
            if not ip_val:
                self.send_message(chat_id, "⚠️ لطفاً آدرس آی‌پی خود را بنویسید، مثلاً:\n<code>/setip 5.120.30.40</code>")
                return
            try:
                self.api.update_client(client_id, {"allowed_ips": [ip_val]})
                self.send_message(chat_id, f"✅ آی‌پی جدید شما (<code>{ip_val}</code>) با موفقیت ثبت شد!", self.main_customer_keyboard(is_admin_user))
            except Exception as e:
                self.send_message(chat_id, f"❌ خطا در ثبت آی‌پی: {e}")
            return

        # Handle /link
        if text.startswith("/link"):
            parts = text.split(maxsplit=1)
            if len(parts) < 2:
                self.send_message(chat_id, "ℹ️ نحوه اتصال اشتراک به ربات:\n<code>/link TOKEN_OR_UUID</code>")
                return
            token_query = parts[1].strip()
            try:
                clients = self.api.list_clients()
                found = next((c for c in clients if c.get("token") == token_query or c.get("uuid") == token_query or c.get("id") == token_query), None)
                if found:
                    self.store.data["user_clients"][str(user_id)] = found["id"]
                    self.store.save()
                    status_data = self.api.get_status()
                    pub_ip = status_data.get("public_ip", "127.0.0.1")
                    self.send_message(chat_id, "✅ <b>اشتراک شما با موفقیت به این حساب متصل شد!</b>\n\n" + self.format_client_card(found, pub_ip), self.main_customer_keyboard(is_admin_user))
                else:
                    self.send_message(chat_id, "❌ اشتراکی با این مشخصات یافت نشد. توکن یا UUID را مجدداً بررسی فرمایید.")
            except Exception as e:
                self.send_message(chat_id, f"❌ خطا: {e}")
            return

        # Commands
        if text.startswith("/start"):
            welcome = (
                f"⚡ <b>به ربات فروش و مدیریت اشتراک HyperDNS خوش آمدید!</b>\n\n"
                f"🚀 با سرویس SmartDNS ما، تحریم‌ها و اختلالات اینترنتی را در بازی‌ها، کنسول‌ها و تمام دیوایس‌ها با <b>پایین‌ترین پینگ و بدون افت سرعت</b> دور بزنید.\n\n"
                f"از گزینه‌های زیر جهت دریافت اکانت تست، خرید یا مدیریت اکانت خود استفاده کنید:"
            )
            self.send_message(chat_id, welcome, self.main_customer_keyboard(is_admin_user))

        elif text.startswith("/admin") and is_admin_user:
            self.send_message(chat_id, "⚙️ <b>پنل کنترل و مدیریت سرور HyperDNS:</b>", self.admin_keyboard())

        elif text.startswith("/status") and is_admin_user:
            try:
                st = self.api.get_status()
                tele = st.get("telemetry", {})
                v = st.get("version", {})
                msg = (
                    f"⚡ <b>وضعیت زنده سرور HyperDNS</b>\n"
                    f"━━━━━━━━━━━━━━━━━━━\n"
                    f"• <b>نسخه:</b> <code>{v.get('display', 'v2.2.0')}</code>\n"
                    f"• <b>آی‌پی سرور:</b> <code>{st.get('public_ip', '127.0.0.1')}</code>\n"
                    f"• <b>تعداد کوئری در ثانیه (QPS):</b> <code>{tele.get('qps', 0):.1f}</code>\n"
                    f"• <b>کل کوئری‌ها:</b> <code>{tele.get('total_queries', 0):,}</code>\n"
                    f"• <b>نرخ کش (Hit Rate):</b> <code>{tele.get('cache_hit_rate', 0):.1f}%</code>\n"
                    f"• <b>مصرف حافظه رم:</b> <code>{tele.get('ram_usage_mb', 0):.1f} MB</code>\n"
                    f"• <b>مصرف پردازنده (CPU):</b> <code>{tele.get('cpu_usage', 0):.1f}%</code>\n"
                )
                self.send_message(chat_id, msg, self.admin_keyboard())
            except Exception as e:
                self.send_message(chat_id, f"❌ خطا: {e}")

        elif text.startswith("/broadcast") and is_admin_user:
            broadcast_msg = text.replace("/broadcast", "").strip()
            if not broadcast_msg:
                self.send_message(chat_id, "ℹ️ لطفاً متن پیام همگانی را بنویسید:\n<code>/broadcast سلام به همه...</code>")
                return
            count = 0
            for uid in self.store.data["bot_users"]:
                try:
                    self.send_message(uid, f"📢 <b>اطلاعیه HyperDNS:</b>\n\n{broadcast_msg}")
                    count += 1
                except Exception:
                    pass
            self.send_message(chat_id, f"✅ پیام همگانی به {count} کاربر ارسال گردید.")

        elif text.startswith("/add") and is_admin_user:
            parts = text.split()
            name = parts[1] if len(parts) > 1 else f"User_{int(time.time())%1000}"
            days = int(parts[2]) if len(parts) > 2 and parts[2].isdigit() else 30
            limit_gb = float(parts[3]) if len(parts) > 3 else 0.0
            max_dev = int(parts[4]) if len(parts) > 4 and parts[4].isdigit() else 2

            try:
                c = self.api.create_client(name=name, days=days, traffic_gb=limit_gb, max_devices=max_dev)
                status_data = self.api.get_status()
                pub_ip = status_data.get("public_ip", "127.0.0.1")
                card = "✅ <b>مشترک جدید با موفقیت ساخته شد!</b>\n\n" + self.format_client_card(c, pub_ip)
                self.send_message(chat_id, card, self.admin_keyboard())
            except Exception as e:
                self.send_message(chat_id, f"❌ خطا در ساخت کاربر: {e}")

        elif text.startswith("/flush") and is_admin_user:
            try:
                if self.api.flush_cache():
                    self.send_message(chat_id, "🧹 <b>کش تمام شارد‌های سرور با موفقیت تخلیه شد!</b>")
                else:
                    self.send_message(chat_id, "⚠️ خطا در پاکسازی کش.")
            except Exception as e:
                self.send_message(chat_id, f"❌ خطا: {e}")

    def process_callback_query(self, query: dict):
        query_id = query.get("id")
        data = query.get("data", "")
        message = query.get("message", {})
        chat_id = message.get("chat", {}).get("id")
        user_id = query.get("from", {}).get("id", chat_id)
        is_admin_user = self.is_admin(chat_id)

        self.answer_callback(query_id)

        if data == "menu_main":
            welcome = "🏠 <b>منوی اصلی ربات HyperDNS:</b>\n\nلطفاً یکی از بخش‌های زیر را انتخاب کنید:"
            self.send_message(chat_id, welcome, self.main_customer_keyboard(is_admin_user))

        elif data == "menu_buy":
            msg = (
                f"🛍 <b>انتخاب و خرید اشتراک هوشمند HyperDNS</b>\n"
                f"━━━━━━━━━━━━━━━━━━━\n"
                f"تمامی پلان‌ها دارای آی‌پی اختصاصی سرور، قابلیت دور زدن کامل تحریم بازی‌ها، "
                f"بدون افت پینگ، و مجهز به پورتال مدیریت اختصاصی دامنه‌ها و زیر‌دامنه‌ها هستند.\n\n"
                f"👇 لطفاً پلان موردنظر خود را انتخاب فرمایید:"
            )
            self.send_message(chat_id, msg, self.plans_keyboard())

        elif data.startswith("plan_"):
            plan_id = data.replace("plan_", "")
            plan = next((p for p in PLANS if p["id"] == plan_id), None)
            if not plan:
                self.send_message(chat_id, "⚠️ پلان موردنظر یافت نشد.")
                return

            msg = (
                f"💎 <b>{plan['name']}</b>\n"
                f"━━━━━━━━━━━━━━━━━━━\n"
                f"📝 <b>ویژگی‌ها:</b> {plan['desc']}\n"
                f"⏳ <b>مدت اعتبار:</b> {plan['days']} روز\n"
                f"📱 <b>محدودیت دستگاه متصل:</b> {plan['max_devices']} دستگاه همزمان\n\n"
                f"💳 <b>قیمت ریالی:</b> <b>{plan['price_toman']}</b>\n"
                f"🪙 <b>قیمت کریپتو:</b> <b>{plan['price_crypto']}</b>\n\n"
                f"📋 <b>اطلاعات واریز:</b>\n"
                f"• شماره کارت: <code>{PAYMENT_CARD_NUMBER}</code>\n"
                f"• به نام: <b>{PAYMENT_CARD_HOLDER}</b>\n"
                f"• کیف پول تتر TRC20:\n<code>{PAYMENT_CRYPTO_WALLET}</code>\n\n"
                f"پس از واریز، دکمهٔ «ثبت و ارسال رسید پرداخت» را در زیر بزنید:"
            )
            self.send_message(chat_id, msg, self.plan_payment_keyboard(plan_id))

        elif data.startswith("pay_receipt_"):
            plan_id = data.replace("pay_receipt_", "")
            self.store.set_state(user_id, f"waiting_receipt:{plan_id}")
            msg = (
                f"📸 <b>ارسال رسید پرداخت:</b>\n\n"
                f"لطفاً تصویر رسید واریزی، یا متن شماره پیگیری / ترنزکشن را در قالب یک پیام ارسال کنید تا برای اپراتور ارسال شود."
            )
            self.send_message(chat_id, msg)

        elif data == "menu_trial":
            # 1-Click Free Trial — use panel config if available
            trial_enabled = getattr(self, '_panel_trial_enabled', True)
            trial_days = getattr(self, '_panel_trial_days', 1)
            trial_gb = getattr(self, '_panel_trial_gb', 2.0)

            if not trial_enabled:
                self.send_message(
                    chat_id,
                    "⚠️ <b>اکانت تست رایگان در حال حاضر غیرفعال است.</b>\n\n"
                    "جهت دریافت اشتراک از بخش «خرید اشتراک» اقدام فرمایید.",
                    self.main_customer_keyboard(is_admin_user)
                )
                return

            if str(user_id) in self.store.data["trials"]:
                self.send_message(
                    chat_id,
                    "⚠️ <b>شما قبلاً اکانت تست رایگان دریافت کرده‌اید!</b>\n\n"
                    "هر کاربر مجاز به دریافت ۱ بار اکانت تست رایگان است. جهت ادامه می‌توانید از بخش «خرید اشتراک» پلان موردنظر خود را تهیه فرمایید.",
                    self.main_customer_keyboard(is_admin_user)
                )
                return

            try:
                trial_name = f"Trial_{user_id}_{int(time.time())%1000}"
                client = self.api.create_client(name=trial_name, days=trial_days, traffic_gb=trial_gb, max_devices=1, note=f"Free Trial via Bot for TG:{user_id}")
                self.store.data["trials"][str(user_id)] = time.time()
                self.store.data["user_clients"][str(user_id)] = client["id"]
                self.store.save()

                status_data = self.api.get_status()
                pub_ip = status_data.get("public_ip", "127.0.0.1")
                card = (
                    f"🎁 <b>تبریک! اکانت تست رایگان {trial_days} روزه شما فعال شد:</b>\n\n" +
                    self.format_client_card(client, pub_ip) +
                    "\n<i>جهت تست، کافی است آدرس Primary DNS را در کنسول، سیستم یا مودم خود وارد فرمایید.</i>"
                )
                self.send_message(chat_id, card, self.main_customer_keyboard(is_admin_user))
            except Exception as e:
                self.send_message(chat_id, f"❌ خطا در ساخت اکانت تست: {e}")

        elif data == "menu_mysub":
            client_id = self.store.data["user_clients"].get(str(user_id))
            if not client_id:
                msg = (
                    "ℹ️ <b>شما هنوز اکانت فعالی به ربات متصل نکرده‌اید!</b>\n\n"
                    "اگر اشتراک دارید، با دستور زیر آن را متصل کنید:\n"
                    "<code>/link TOKEN_OR_UUID</code>\n\n"
                    "یا اگر می‌خواهید تست کنید، دکمهٔ «اکانت تست رایگان» را بزنید."
                )
                self.send_message(chat_id, msg, self.main_customer_keyboard(is_admin_user))
                return
            try:
                c = self.api.get_client(client_id)
                status_data = self.api.get_status()
                pub_ip = status_data.get("public_ip", "127.0.0.1")
                self.send_message(chat_id, self.format_client_card(c, pub_ip), self.main_customer_keyboard(is_admin_user))
            except Exception as e:
                self.send_message(chat_id, f"❌ خطا در بازیابی اطلاعات اشتراک: {e}")

        elif data == "menu_update_ip":
            self.store.set_state(user_id, "waiting_ip")
            self.send_message(
                chat_id,
                "🌐 <b>ثبت یا تغییر آی‌پی:</b>\n\n"
                "لطفاً آدرس IP عمومی فعلی خود را ارسال کنید (مثلاً: <code>5.120.30.40</code>)\n"
                "همچنین می‌توانید وارد لینک پورتال خود شده و دکمهٔ «ثبت آی‌پی من» را بزنید."
            )

        elif data == "menu_guides":
            self.send_message(chat_id, "📚 <b>راهنمای تنظیم DNS روی دیوایس‌های گوناگون:</b>", self.guides_keyboard())

        elif data == "guide_ps":
            self.send_message(
                chat_id,
                "🎮 <b>آموزش تنظیم DNS روی پلی‌استیشن (PS4/PS5):</b>\n\n"
                "1. وارد Settings ➔ Network شوید.\n"
                "2. گزینهٔ Set Up Internet Connection را انتخاب کنید.\n"
                "3. شبکه خود (Wi-Fi یا LAN) را انتخاب کرده و روی Custom بگذارید.\n"
                "4. آی‌پی را Automatic و DHCP را Do Not Specify بگذارید.\n"
                "5. در DNS Settings حالت Manual را انتخاب کرده و مقادیر سرور را وارد کنید.\n"
                "6. در پایان Test Connection بگیرید.",
                self.guides_keyboard()
            )

        elif data == "guide_xbox":
            self.send_message(
                chat_id,
                "🎮 <b>آموزش تنظیم DNS روی ایکس‌باکس:</b>\n\n"
                "1. دکمه Xbox را زده و Settings را باز کنید.\n"
                "2. به مسیر General ➔ Network settings ➔ Advanced settings بروید.\n"
                "3. گزینهٔ DNS settings را روی Manual بگذارید.\n"
                "4. آدرس سرور DNS را در فیلدهای Primary و Secondary وارد و ذخیره فرمایید.",
                self.guides_keyboard()
            )

        elif data == "guide_win":
            self.send_message(
                chat_id,
                "💻 <b>آموزش تنظیم DNS روی ویندوز ۱۰ و ۱۱:</b>\n\n"
                "1. وارد Settings ➔ Network & Internet شوید.\n"
                "2. روی کانکشن متصل (Wi-Fi یا Ethernet) کلیک کنید.\n"
                "3. در بخش DNS server assignment دکمهٔ Edit را بزنید.\n"
                "4. حالت را Manual کرده، IPv4 را روشن و آدرس‌های DNS را وارد نمایید.",
                self.guides_keyboard()
            )

        elif data == "guide_mobile":
            self.send_message(
                chat_id,
                "📱 <b>آموزش تنظیم روی موبایل:</b>\n\n"
                "• <b>اندروید (روش Private DNS):</b>\n"
                "مسیر Settings ➔ Network & internet ➔ Private DNS ➔ وارد کردن نام دامنه اختصاصی.\n\n"
                "• <b>آیفون (iOS):</b>\n"
                "مسیر Settings ➔ Wi-Fi ➔ آیکن (i) شبکه متصل ➔ Configure DNS ➔ Manual ➔ افزودن آدرس سرور.",
                self.guides_keyboard()
            )

        elif data == "guide_router":
            self.send_message(
                chat_id,
                "🌐 <b>آموزش تنظیم روی مودم و روتر:</b>\n\n"
                "1. با مرورگر وارد پنل مودم شوید (192.168.1.1).\n"
                "2. به بخش WAN یا DHCP بروید و DNS را روی Static قرار دهید.\n"
                "3. آدرس DNS را وارد و مودم را یک‌بار ری‌استارت کنید تا تمام دستگاه‌ها اتوماتیک متصل شوند.",
                self.guides_keyboard()
            )

        elif data == "menu_support":
            msg = (
                f"📞 <b>پشتیبانی و سوالات:</b>\n\n"
                f"جهت راهنمایی، پیگیری سفارش، مشاوره و پشتیبانی فنی می‌توانید مستقیماً با ادمین در ارتباط باشید:\n\n"
                f"👤 ادمین پشتیبانی: <b>{SUPPORT_ADMIN_USERNAME}</b>"
            )
            self.send_message(chat_id, msg, self.main_customer_keyboard(is_admin_user))

        # Admin Callbacks
        elif data == "menu_admin" and is_admin_user:
            self.send_message(chat_id, "⚙️ <b>پنل مدیریت سرور:</b>", self.admin_keyboard())

        elif data == "admin_status" and is_admin_user:
            try:
                st = self.api.get_status()
                tele = st.get("telemetry", {})
                v = st.get("version", {})
                msg = (
                    f"⚡ <b>تلمتری سرور HyperDNS</b>\n"
                    f"• نسخه: <code>{v.get('display', '')}</code>\n"
                    f"• آی‌پی: <code>{st.get('public_ip', '')}</code>\n"
                    f"• QPS: <code>{tele.get('qps', 0):.1f}</code>\n"
                    f"• Total Queries: <code>{tele.get('total_queries', 0):,}</code>\n"
                    f"• Cache Hit: <code>{tele.get('cache_hit_rate', 0):.1f}%</code>\n"
                    f"• CPU: <code>{tele.get('cpu_usage', 0):.1f}%</code>\n"
                    f"• RAM: <code>{tele.get('ram_usage_mb', 0):.1f} MB</code>\n"
                )
                self.send_message(chat_id, msg, self.admin_keyboard())
            except Exception as e:
                self.send_message(chat_id, f"❌ خطا: {e}")

        elif data == "admin_clients" and is_admin_user:
            try:
                clients = self.api.list_clients()
                msg = f"👥 <b>تعداد کل مشترکین فعال: {len(clients)}</b>\n\n"
                for c in clients[:10]:
                    name = html.escape(str(c.get('display_name') or c.get('name')))
                    dev_cnt = len(c.get('allowed_ips', []))
                    max_d = c.get('max_devices', 2)
                    msg += f"• <b>{name}</b> (ID: <code>{c.get('id')}</code>) — {dev_cnt}/{max_d} دیوایس\n"
                self.send_message(chat_id, msg, self.admin_keyboard())
            except Exception as e:
                self.send_message(chat_id, f"❌ خطا: {e}")

        elif data == "admin_pending_orders" and is_admin_user:
            pending = [o for o in self.store.data["orders"].values() if o.get("status") == "pending"]
            if not pending:
                self.send_message(chat_id, "ℹ️ هیچ سفارش در انتظار تاییدی وجود ندارد.", self.admin_keyboard())
                return
            for o in pending:
                admin_msg = (
                    f"📦 <b>سفارش:</b> <code>{o['order_id']}</code>\n"
                    f"• کاربر: <code>{o['user_id']}</code> (@{o.get('username', '')})\n"
                    f"• پلان: {o['plan_name']}\n"
                    f"• رسید: {html.escape(str(o.get('receipt', '')))}\n"
                )
                admin_markup = {
                    "inline_keyboard": [
                        [
                            {"text": "✅ تایید و صدور اکانت", "callback_data": f"approve_{o['order_id']}"},
                            {"text": "❌ رد سفارش", "callback_data": f"reject_{o['order_id']}"}
                        ]
                    ]
                }
                self.send_message(chat_id, admin_msg, admin_markup)

        elif data.startswith("approve_") and is_admin_user:
            order_id = data.replace("approve_", "")
            order = self.store.data["orders"].get(order_id)
            if not order or order.get("status") != "pending":
                self.send_message(chat_id, "⚠️ این سفارش قبلاً بررسی شده یا نامعتبر است.")
                return

            plan_id = order.get("plan_id")
            plan = next((p for p in PLANS if p["id"] == plan_id), PLANS[0])
            cust_uid = order["user_id"]
            cust_chat = order["chat_id"]

            try:
                # Provision account
                cust_name = f"User_{cust_uid}_{int(time.time())%1000}"
                client = self.api.create_client(
                    name=cust_name,
                    days=plan["days"],
                    traffic_gb=plan["traffic_gb"],
                    max_devices=plan["max_devices"],
                    note=f"Order {order_id} via Bot"
                )
                order["status"] = "approved"
                order["client_id"] = client["id"]
                self.store.data["user_clients"][str(cust_uid)] = client["id"]
                self.store.save()

                status_data = self.api.get_status()
                pub_ip = status_data.get("public_ip", "127.0.0.1")

                # Notify Admin
                self.send_message(chat_id, f"✅ سفارش <code>{order_id}</code> تایید شد و اشتراک مشتری با موفقیت صادر گردید!")

                # Send credentials to customer
                cust_card = (
                    "🎉 <b>سفارش شما تایید و اشتراک هوشمندتان فعال گردید!</b>\n\n" +
                    self.format_client_card(client, pub_ip)
                )
                self.send_message(cust_chat, cust_card, self.main_customer_keyboard(False))
            except Exception as e:
                self.send_message(chat_id, f"❌ خطا در صدور اکانت: {e}")

        elif data.startswith("reject_") and is_admin_user:
            order_id = data.replace("reject_", "")
            order = self.store.data["orders"].get(order_id)
            if order and order.get("status") == "pending":
                order["status"] = "rejected"
                self.store.save()
                self.send_message(chat_id, f"❌ سفارش <code>{order_id}</code> رد شد.")
                self.send_message(
                    order["chat_id"],
                    "❌ <b>سفارش شما تایید نشد!</b>\nرسید ارسال‌شده معتبر تشخیص داده نشد. لطفاً جهت بررسی مجدد به پشتیبانی پیام دهید."
                )

        elif data == "admin_flush" and is_admin_user:
            try:
                if self.api.flush_cache():
                    self.send_message(chat_id, "🧹 <b>کش تمام شارد‌های سرور با موفقیت تخلیه شد!</b>")
                else:
                    self.send_message(chat_id, "⚠️ خطا در پاکسازی کش.")
            except Exception as e:
                self.send_message(chat_id, f"❌ خطا: {e}")

    def run_poll_iteration(self):
        """Poll Telegram updates once (non-blocking testable loop)."""
        url = f"{self.api_url}/getUpdates?offset={self.last_update_id + 1}&timeout=1"
        try:
            r = requests.get(url, timeout=5)
            if r.status_code == 200:
                data = r.json()
                if data.get("ok"):
                    for update in data.get("result", []):
                        self.last_update_id = update.get("update_id", self.last_update_id)
                        if "message" in update:
                            self.process_message(update["message"])
                        elif "callback_query" in update:
                            self.process_callback_query(update["callback_query"])
        except Exception as e:
            logger.debug(f"Polling update error (expected if network isolated): {e}")


def fetch_panel_config(api_base: str, api_key: str) -> dict:
    """
    Fetch Telegram bot settings from the HyperDNS panel API.
    Returns the config dict on success, or empty dict on failure.
    Endpoint: GET /api/v2/config/telegram (API-key authenticated).
    """
    url = f"{api_base.rstrip('/')}/config/telegram"
    headers = {"X-API-Key": api_key, "Content-Type": "application/json"}
    try:
        r = requests.get(url, headers=headers, timeout=10)
        if r.status_code == 200:
            data = r.json()
            if data.get("success") and data.get("config"):
                return data["config"]
        logger.warning(f"Panel config fetch returned status {r.status_code}: {r.text[:200]}")
    except Exception as e:
        logger.warning(f"Could not fetch config from panel API ({url}): {e}")
    return {}


if __name__ == "__main__":
    # -------------------------------------------------------------------------
    # Configuration resolution order:
    #   1. HyperDNS Panel API (/api/v2/config/telegram)  — preferred
    #   2. Environment variables                          — fallback
    # -------------------------------------------------------------------------

    # Step 0: We always need the API base URL and API key from env to talk
    # to HyperDNS at all. These two cannot come from the panel.
    api_base = os.getenv("HYPERDNS_API_BASE", "http://127.0.0.1:8080/api/v2").strip()
    api_key = os.getenv("HYPERDNS_API_KEY", "").strip()
    if not api_key:
        sys.exit(
            "[FATAL] HYPERDNS_API_KEY is not set.\n"
            "        Set it in the environment, your systemd unit, or a .env file:\n"
            "            export HYPERDNS_API_KEY=hdns_live_...\n"
            "        The bot needs this key to communicate with the HyperDNS REST API."
        )

    # Step 1: Try to pull the full config from the panel database.
    panel_cfg = fetch_panel_config(api_base, api_key)

    if panel_cfg:
        logger.info("✓ Loaded Telegram bot configuration from the HyperDNS panel database.")
        if not panel_cfg.get("enabled", False):
            sys.exit(
                "[INFO] Telegram bot integration is DISABLED in the panel settings.\n"
                "       Enable it from the HyperDNS Dashboard → Settings → Telegram Bot card."
            )
        token = panel_cfg.get("bot_token", "").strip()
        admin_ids_raw = panel_cfg.get("admin_chat_ids", "").strip()

        # Override globals from panel config
        if panel_cfg.get("card_number"):
            globals()["PAYMENT_CARD_NUMBER"] = panel_cfg["card_number"]
        if panel_cfg.get("card_holder"):
            globals()["PAYMENT_CARD_HOLDER"] = panel_cfg["card_holder"]
        if panel_cfg.get("support_username"):
            globals()["SUPPORT_ADMIN_USERNAME"] = panel_cfg["support_username"]

        # Override trial settings in PLANS[0] if trial is configured
        if panel_cfg.get("trial_enabled") is not None:
            # Trial configuration is handled in the free trial callback
            pass
        if panel_cfg.get("monthly_price_toman"):
            PLANS[0]["price_toman"] = panel_cfg["monthly_price_toman"]

    else:
        logger.warning("⚠ Could not load config from panel API; falling back to environment variables.")
        token = os.getenv("TELEGRAM_BOT_TOKEN", "").strip()
        admin_ids_raw = os.getenv("TELEGRAM_ADMIN_CHAT_IDS", "").strip()

    # Step 2: Validate essential credentials.
    if not token:
        sys.exit(
            "[FATAL] TELEGRAM_BOT_TOKEN is not configured.\n"
            "        Set it in the HyperDNS Dashboard → Settings → Telegram Bot card,\n"
            "        or set TELEGRAM_BOT_TOKEN in the environment."
        )

    admin_ids = parse_admin_chat_ids(admin_ids_raw)

    # Step 3: Initialize API client and bot instance.
    client = HyperDNSClient(api_base, api_key)
    bot = HyperDNSTelegramBot(token, client, admin_ids)

    # Inject panel trial config into bot instance for runtime use
    if panel_cfg:
        bot._panel_trial_days = panel_cfg.get("trial_days", 1)
        bot._panel_trial_gb = panel_cfg.get("trial_traffic_gb", 2.0)
        bot._panel_trial_enabled = panel_cfg.get("trial_enabled", True)

    logger.info("HyperDNS Sales & Management Telegram Bot initialized successfully.")
    logger.info("Serving customers and %d admin(s); polling for updates...", len(admin_ids))
    if panel_cfg:
        logger.info("Config source: HyperDNS Panel Database | Support: %s", SUPPORT_ADMIN_USERNAME)
    else:
        logger.info("Config source: Environment Variables")

    try:
        while True:
            bot.run_poll_iteration()
            time.sleep(1)
    except KeyboardInterrupt:
        logger.info("Shut down by operator.")
