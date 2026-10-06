# GreFlow — راهنمای فارسی

GreFlow بین دو سرور لینوکس یک تانل GRE می‌سازد و پورت‌های TCP/UDP سرور ورودی را به سرویس سرور خارج فوروارد می‌کند. پیش‌فرض آدرس داخلی `10.77.0.0/30` و MTU برابر `1476` است. اطلاعات سرورهای آزمایش قبلی در کد قرار داده نشده‌اند.

**GRE رمزنگاری ندارد.** رمزنگاری باید در سرویس شما یا لایه‌ای جدا انجام شود. پروتکل IP شمارهٔ **۴۷** باید در مسیر و فایروال ارائه‌دهنده مجاز باشد؛ منظور پورت ۴۷ نیست.

## نصب روی هر دو سرور

```bash
apt-get update
apt-get install -y curl ca-certificates iproute2 iptables procps iputils-ping conntrack
bash <(curl -fLsS https://raw.githubusercontent.com/TIR3D4/GreFlow/main/install.sh)
```

نصاب باینری انتشار `v0.2.0` را از همین مخزن دریافت و checksum را بررسی می‌کند. انتشار پس از موفقیت CI ساخته می‌شود. برای بررسی قبل از اجرا، فایل نصب را دانلود و بخوانید؛ [راهنمای انگلیسی](../README.md) روش ساخت از سورس را هم دارد.

## سرور ایران / ورودی

به‌جای `ENTRY_IPV4` و `EXIT_IPV4` آدرس واقعی سرورهای خودتان را بگذارید:

```bash
greflow setup --role entry --local ENTRY_IPV4 --remote EXIT_IPV4
greflow add tcp 443 2020
```

## سرور خارج / مقصد

```bash
greflow setup --role exit --local EXIT_IPV4 --remote ENTRY_IPV4
greflow add tcp 2020
```

روی خارج، سرویس Xray یا برنامهٔ مقصد باید روی `0.0.0.0:2020` یا `10.77.0.2:2020` گوش بدهد. گوش‌دادن فقط روی `127.0.0.1` کافی نیست. دستور `add` روی خارج پورت سرویس را در INPUT مجاز می‌کند؛ NAT روی ایران انجام می‌شود. بعد کلاینت را به `ENTRY_IPV4:443` وصل کنید.

```bash
greflow test
greflow doctor
# فقط پس از اتصال واقعی کلاینت:
greflow verify yes
```

## منو و مدیریت

```bash
greflow
greflow add tcp 2020
greflow add udp 1194
greflow add udp 10000-20000
greflow list
greflow remove tcp 2020
greflow status
greflow stats
greflow repair
greflow restart
greflow logs
greflow tune
```

پورت‌های مقصد UDP و بازه‌ها را روی خارج هم اضافه کنید. بازه‌ها فقط با همان شمارهٔ پورت در مقصد پشتیبانی می‌شوند؛ بازهٔ انتقالی در نسخهٔ اول رد می‌شود. بازهٔ بزرگ همهٔ سرویس‌های آن بازه را در معرض ورودی می‌گذارد؛ فقط مقدار لازم را باز کنید.

تانل و قوانین با `greflow.service` پس از ریبوت بازسازی می‌شوند. `apply` و `repair` فقط رابط و chainهای متعلق به GreFlow را بازسازی می‌کنند، شمارنده‌ها صفر می‌شوند و ممکن است اتصال‌های فعال قطع شوند. `down` داده را متوقف می‌کند ولی سرویس فعال در بوت بعدی دوباره آن را می‌سازد.

سلامت در لایه‌های H0 تا H6 گزارش می‌شود. UP بودن رابط به معنی اتصال واقعی نیست. TCP مقصد بررسی می‌شود؛ برای UDP باید برنامهٔ واقعی را تست کرد. تأیید کاربر فقط با `verify yes` ثبت می‌شود. `tune` تنها وضعیت و پیشنهاد را نمایش می‌دهد و تنظیم کرنل را تغییر نمی‌دهد.

برای حذف:

```bash
greflow uninstall --yes
```

فقط اجزای GreFlow حذف می‌شوند؛ لاگ‌ها و پشتیبان‌های بازیابی می‌مانند. مقادیر عمومی sysctl در صورتی برگردانده می‌شوند که بعداً ابزار دیگری آن‌ها را تغییر نداده باشد.

پیش‌نیاز: دسترسی root، systemd و IPv4 مستقیماً روی سرور. نصب اولیه برای Ubuntu 24.04 هدف‌گذاری شده است. تانل در شبکهٔ کانتینری بدون مجوز مدیریتی و سرور پشت NAT در نسخهٔ اول پشتیبانی نمی‌شود. برای مشکل اتصال اول IP دو طرف، اجازهٔ GRE، listener خارج، پورت فایروال ارائه‌دهنده و MTU را بررسی کنید.

## یک ایران و چند خارج — نسخهٔ ۰٫۲

تانل قبلی شما با نام `default` و همان فایل‌ها باقی می‌ماند. دستورهای قبلی همچنان برای آن کار می‌کنند. تانل دوم را با نامی مثل `exit2` بسازید؛ رابط، شبکهٔ داخلی، فایروال و سرویس آن مستقل است.

روی ایران:

```bash
greflow --tunnel exit2 setup --role entry --local ENTRY_IPV4 --remote SECOND_EXIT_IPV4 --network 10.77.0.4/30
greflow --tunnel exit2 add tcp 3212 443
systemctl start greflow-exit2.service
```

روی خارج دوم:

```bash
greflow --tunnel exit2 setup --role exit --local SECOND_EXIT_IPV4 --remote ENTRY_IPV4 --network 10.77.0.4/30
greflow --tunnel exit2 add tcp 443
systemctl start greflow-exit2.service
```

IP واقعی را جایگزین کنید. شبکهٔ جدید روی هر دو طرف باید یکسان باشد. برای هر تانل شبکه‌ای جدا لازم است؛ این مثال از `10.77.0.4/30` و آدرس‌های `10.77.0.5` و `10.77.0.6` استفاده می‌کند. تنظیم نام‌دار با آرگومان‌ها نیاز به `--network` صریح دارد.

اگر `3212` از قبل در تانل اول ثبت شده، اول `greflow remove tcp 3212` را اجرا کنید و سپس آن را به تانل دوم اضافه کنید. یک پورت TCP روی یک IP ایران نمی‌تواند هم‌زمان به دو خارج تخصیص داده شود؛ TCP و UDP جدا هستند. این قابلیت انتخاب مقصد با پورت است؛ تقسیم بار یا جابه‌جایی خودکار ندارد.

```bash
greflow tunnels
greflow --tunnel exit2 list
greflow --tunnel exit2 test
greflow --tunnel exit2 repair
greflow --tunnel exit2 restart
greflow --tunnel exit2 uninstall --yes
```

حذف تانل دوم به تنظیمات تانل اول دست نمی‌زند. باینری تا وقتی تانل دیگری پیکربندی شده باقی می‌ماند. مقادیر عمومی کرنل فقط هنگام توقف آخرین تانل ورودی برگردانده می‌شوند. منو گزینهٔ فهرست تانل‌ها و انتخاب/ساخت تانل نام‌دار دارد.

برای ارتقا، نصب‌کننده را دوباره اجرا کنید؛ فایل تنظیم فعلی حفظ می‌شود و تانل‌ها خودکار restart نمی‌شوند:

```bash
curl -fLsS https://raw.githubusercontent.com/TIR3D4/GreFlow/main/install.sh -o /root/install-greflow.sh && bash /root/install-greflow.sh
greflow version
greflow tunnels
```
