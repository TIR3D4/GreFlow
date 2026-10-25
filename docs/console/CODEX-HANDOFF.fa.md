# دستور شروع برای Codex

این فایل را همراه مخزن کامل GreFlow در اختیار Codex قرار دهید. هدف ساخت نسخهٔ قابل انتشار است، نه فقط ماکاپ. اختیار کار روی کد، تست، مستندات و buildها داده شده؛ انتشار روی میزبان واقعی/تنظیم سرورهای کاربران فقط با دستور مشخص مالک انجام شود.

## متن قابل کپی

«پروژهٔ GreFlow موجود را به یک محصول مدیریت تانل با پنل وب فارسی ارتقا بده. در همین مخزن کار کن. پیش از تغییر، وضعیت git، دستورهای AGENTS.md و موتور فعلی را بخوان. فایل‌های زیر مرجع نیازمندی‌اند:

- docs/console/PRODUCT.fa.md
- docs/console/ARCHITECTURE.md
- docs/console/ACCEPTANCE.md
- web/console/index.html، styles.css، app.js و domain.js
- docs/console/references/

رابط HTML/CSS/JavaScript موجود یک پیش‌نمایش تعاملی با دادهٔ نمونه است. سبک و جریان‌های آن را مبنا بگیر، اما backend واقعی، ذخیره‌سازی، کنترل دسترسی، عملیات پایدار، agent/SSH و اتصال به موتور Go را بساز. آن را صرفاً با API ساختگی به‌عنوان محصول کامل تحویل نده. از عبارت Java در این پروژه منظور JavaScript مرورگر است، نه JVM.

نسخهٔ Windows محلی و Linux سرور باید یک محصول و یک رابط مشترک باشند. control plane روی Windows عملیات kernel اجرا نمی‌کند؛ آن‌ها به عامل/dispatcher لینوکسی منتقل می‌شوند. در حالت محلی، SSH بدون نیاز به endpoint عمومی کاربرد دارد. در حالت عامل خروجی، نیاز به HTTPS قابل دسترس را واقعی پیاده‌سازی و توضیح بده.

CLI v0.2.0 و تانل‌های نصب‌شده نباید با اجرای پنل تغییر یا قطع شوند. discovery اول read-only باشد. موتور فعلی را بدون بررسی fork/بازنویسی نکن؛ typed adapter اضافه کن. خروجی JSON پایدار برای health/inventory بساز، بدون شکستن stdout فعلی. قوانین فعلی مانند عدم تکرار unkeyed endpoint pair، private /30 و هویت range را حفظ کن.

همهٔ بخش‌ها: bootstrap/login/TOTP؛ نمای کلی؛ ایران‌ها و خارج‌ها با چند IP؛ کارت/جدول تانل؛ نقشهٔ پورت؛ wizard افزودن و تک‌دستور امن خارج؛ ویرایش mapping؛ start/stop/restart/remove؛ انتقال پورت با preflight و rollback؛ تست و لاگ؛ عملیات ماندگار؛ schedule روی ایران؛ هشدار تلگرام؛ بکاپ/restore؛ تنظیمات و RBAC.

RTL کامل، Vazirmatn محلی، زمینهٔ یاسی، کارت سفید گرد، آبی اصلی، آیکون خطی، glass ملایم فقط کنترل‌ها، light/dark/system و responsive. تصاویر مرجع raster هستند و داده‌هایشان معیار صحت نیست. متن، دسترس‌پذیری و اطلاعات نسخهٔ عملی باید دقیق باشند. مسیرهای فنی LTR؛ نام وضعیت‌ها همراه آیکون؛ stale/unknown از healthy جدا.

ترتیب پیشنهادی اجرا:
۱. نقشهٔ gap کوتاه و تصمیم‌های فنی؛ سپس بلافاصله پیاده‌سازی.
۲. تفکیک platform و structured discovery/health + contract tests.
۳. control plane، DB migrations، auth، RBAC و secret storage.
۴. typed agent و strict SSH dispatcher، enrollment و inventory import.
۵. frontend واقعی و operations/events، بدون دکمهٔ نمایشی در production.
۶. plan/apply/journal و recovery، mapping transfer و schedule.
۷. هشدار، history retention، backup/restore و packaging.
۸. تکمیل acceptance tests، Windows/Linux CI، دستورات نصب و release notes.

برای هر مرحله قابلیت واقعی و شواهد تست بده. هیچ سرور یا IP واقعی داخل دادهٔ اولیهٔ production seed نکن؛ نمونه‌ها فقط demo mode صریح. secret در localStorage، log یا command history ذخیره نکن. generic shell/terminal API و firewall flush عمومی نساز. برای ادعای قابل انتشار بودن باید تمام معیارهای docs/console/ACCEPTANCE.md پاس شوند. اگر محیط برای یک تست لازم نیست، دقیقاً آن gate را blocked بنویس و ادعای موفقیت نکن.

در پایان سورس، buildهای قابل نصب پلتفرم‌های پشتیبانی‌شده، تست‌ها، مستندات اجرا، migration و rollback تحویل بده. شماره نسخه/tag/Release را تا عبور gateها منتشر نکن.»

## نکته دربارهٔ نسخهٔ فعلی

این شاخه به‌عمد core یا installer فعلی را تغییر نمی‌دهد. تغییرات شامل رابط مرجع، validatorهای نمونه، تست‌ها و اسناد اجرایی است؛ هنوز API، SSH connector، عامل یا scheduler واقعی ندارد. تصاویر و snapshotهای مرورگر در صورت وجود با برچسب نمایشی استفاده شوند.
