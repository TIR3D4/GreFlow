# GreFlow Console — سند محصول و UI/UX قابل پیاده‌سازی

وضعیت: مشخصات انتشار + رابط مرجع تعاملی، نه پنل متصل به سرور.

## هدف و محدوده

یک محصول واحد برای مدیریت چند ایران و چند خارج بسازید که روی ویندوز محلی و لینوکس همیشه‌روشن اجرا شود. GreFlow v0.2.0 و دستورات فعلی باید سازگار بمانند. پنل باید تانل‌های موجود را وارد مدیریت کند، بدون بازسازی یا قطع اجباری آن‌ها.

نسخهٔ اول قابل انتشار: GRE از طریق موتور همین مخزن. Backhaul و سایر روش‌ها قابلیت آینده‌اند؛ در رابط منتشرشده روش پیاده‌سازی‌نشده نباید قابل انتخاب یا موفق نمایش داده شود.

مرجع کد طراحی: `web/console/`. تصاویر مرجع در `docs/console/references/`. تصاویر برای سبک‌اند؛ مقادیر، کشورها، وضعیت‌ها و نمودارها دادهٔ زنده یا تأیید موقعیت IP نیستند. مشخصات این سند و رفتار موتور بر تصاویر اولویت دارد.

## اصل‌های محصول

- اول مشخص کن کاربر روی کدام ایران است و تغییر روی چه مسیرهایی اثر می‌گذارد.
- desired (ذخیره‌شده)، observed (مشاهده‌شده) و applied (اعمال‌شده) جدا باشند.
- فعال‌بودن systemd به معنی سلامت GRE و TCP یا کانفیگ Xray نیست.
- شکست ICMP همراه موفقیت TCP، «اختلال پینگ» است، نه قطع کامل.
- اگر داده قدیمی شد، وضعیت سبز قبلی را به‌عنوان وضعیت فعلی نشان نده.
- عملیات طولانی روی سرور/صف ادامه یابد؛ بستن مرورگر آن را لغو نکند.
- همهٔ تغییرات مشخص، محدود، قابل پیگیری و با برگشت best-effort باشند؛ تضمین اتمیک کرنل ادعا نشود.
- رابط فارسی/RTL؛ IP، نام فنی، پورت، مسیر، کد و لاگ LTR و ایزوله باشند.

## کاربران و دسترسی

| نقش | مجوز |
|---|---|
| مالک/مدیر | سرورها، دسترسی‌ها، تغییرات، بکاپ و مدیریت کاربران |
| اپراتور | تست، مشاهده، پورت و lifecycle در محدودهٔ تخصیص‌یافته |
| مشاهده‌گر | مشاهدهٔ وضعیت و لاگ پالایش‌شده؛ بدون mutation یا مشاهدهٔ secret |

اجرای محلی نیز login دارد. اولین اجرا از طریق کد یک‌بارمصرف محلی/CLI bootstrap مالک را می‌سازد؛ رمز پیش‌فرض عمومی ممنوع. نسخهٔ لینوکس پشت HTTPS قرار می‌گیرد. TOTP، recovery code و مدیریت نشست برای انتشار پشتیبانی شوند. کلید SSH، توکن عامل و کلید بکاپ در لاگ/مرورگر ذخیره نشوند.

## نقشهٔ صفحات

### شروع، ورود و راه‌اندازی

صفحهٔ login، ایجاد مالک در اولین اجرا، ورود دومرحله‌ای، recovery code، نشست منقضی و عدم دسترسی. راه‌اندازی اولیه شامل زبان، پوسته، منطقه زمانی، روش افزودن اولین ایران است. در اجرای ویندوز مرورگر localhost باز شود و نسخه/محل داده‌ها در About قابل مشاهده باشد.

### نمای کلی

شاخص‌ها: ایران‌های قابل دسترس، تانل‌های healthy/degraded/down/unknown/stale، عملیات pending/running. شمارنده‌ها از همان query و همان زمان snapshot محاسبه شوند؛ تناقض آماری ممنوع. بخش نیازمند توجه شامل مشکل، سرور، زمان، اثر و اقدام مشخص است. کارت ایران نام، IPv4/IPv6 ثبت‌شده، روش مدیریت، heartbeat، منابع، تعداد تانل و دکمهٔ مدیریت دارد.

### سرورها

لیست با فیلتر ایران/خارج، برچسب، دسترسی و روش مدیریت. هر رکورد میزبان می‌تواند چند IP داشته باشد؛ چند IP نباید سه سرور مستقل با عملیات reboot جدا تصور شوند. IP عمومی باید با interfaceهای واقعی تطبیق داده شود؛ NAT-to-private در موتور فعلی پشتیبانی نشده و onboarding باید محدودیت را توضیح دهد.

افزودن با SSH: نام، IP/hostname، پورت، کاربر، کلید یا secret موقت، بررسی اثرانگشت واقعی قبل از اعتماد. کلید host تغییرکرده خطای مستقل است، نباید auto-accept شود. افزودن با agent: دستور enrollment یک‌بارمصرف و منتظر heartbeat؛ لغو و انقضا. پس از اتصال، discovery بدون mutation، پیش‌نمایش configهای خوانده‌شده و واردکردن انتخابی.

تب‌ها: تانل‌ها، پورت‌ها، سلامت، دسترسی، فعالیت‌ها. سلامت شامل CPU/RAM/disk، load، شبکه/DNS/route، conntrack count/max و نسخه‌ها با زمان نمونه است. نبود داده = نامشخص، نه صفر. نرخ ترافیک از اختلاف counter/time محاسبه شود، reset تشخیص داده شود.

### تانل‌ها

کارت/جدول؛ جست‌وجو در نام، alias، IP و پورت؛ فیلتر وضعیت، پروتکل، روش، برچسب و ایران. فیلتر/مرتب‌سازی در URL قابل اشتراک و pagination برای صدها تانل. کارت: نام، endpointها، روش، interface، پورت‌ها با overflow، سلامت لایه‌ای، latency/loss، RX/TX و نرخ با محدودهٔ زمانی، آخرین بررسی/خطا. اکشن اصلی بررسی؛ جزئیات و منوی lifecycle در کنار آن.

جزئیات شامل بررسی‌ها، هر فوروارد، counterها، history، config desired/observed، drift، نصب طرف مقابل، MTU، persistence، لاگ، زمان‌بندی، نگهداری و عملیات اخیر. دادهٔ config خام فقط read-only با redaction؛ ادیت فرم محدود و validated. تغییر endpoint/network موتور فعلی نیازمند uninstall/setup است، نه edit درجا؛ UI باید اثر بر نشست‌ها و plan را نشان دهد.

### پورت‌ها و مسیر واقعی

هر ردیف: endpoint ایران + protocol + public port/range → tunnel → endpoint خارج + destination port/range. desired، rule observed، تست peer، تست مقصد و تأیید واقعی کاربر ستون‌های متفاوت‌اند. تست DNS برای hostname ثبت‌شده A و AAAA را جدا گزارش کند؛ فقط یک lookup برای ادعای همهٔ کاربران کافی نیست.

GreFlow v0.2 فقط mapping range همسان را قبول می‌کند؛ offset range در فرم غیرفعال با علت. TCP و UDP می‌توانند شماره یکسان داشته باشند. reservation با tuple endpoint/protocol/range در تمام configهای حتی متوقف‌شده سنجیده شود. حذف فوروارد، inbound Xray را حذف نمی‌کند. غیرفعال‌بودن firewalls مدیریتی در UI به معنی عبور provider firewall نیست.

### عملیات

لیست و detail با ID، actor، scope، plan، تأثیر، timestamps، پیشرفت مرحله‌ای، stdout پالایش‌شده، خطا و result. retry mutation با idempotency key دوباره اجرا نشود. reconnect مرورگر با cursor رخدادها را ادامه دهد. عملیات با heartbeat ازدست‌رفته «نامشخص/نیازمند تطبیق» شود، نه موفق یا شکست ساختگی. لغو فقط در مرحلهٔ قابل لغو؛ علت غیرفعال‌بودن cancel دیده شود. هیچ POST طولانی به lifecycle صفحه وابسته نباشد.

### زمان‌بندی و هشدارها

برنامه برای هر تانل/ایران: تست دوره‌ای، restart برنامه‌ریزی‌شده، بازیابی شرطی. انتخاب‌های دقیقه/ساعت/روز، timezone و پیش‌نمایش سه اجرای بعدی؛ timezone نمایش را با منطقهٔ کاربر اشتباه نگیرید. آخرین نتیجه، اجرای بعدی، disable، edit و delete. اجرای واقعی systemd timer و companion runner روی خود ایران؛ خاموشی پنل مانع اجرا نیست.

بازیابی پیش‌فرض خاموش: آستانهٔ شکست متوالی، cooldown، سقف تلاش در ساعت، jitter، منع اجرای همزمان و maintenance window. فقط شکست ping باعث restart نشود اگر سرویس مقصد پاسخ می‌دهد. ریبوت کل میزبان اقدام جدا با سطح دسترسی و تأثیر همهٔ تانل‌هاست؛ معادل restart تانل نیست.

هشدار تلگرام با destination مشخص، test notification، secret مخفی، retry/backoff، dedup، اعلان بازیابی، mute/maintenance و ثبت delivery status. عدم ارسال هشدار وقتی پنل ویندوز خاموش است توضیح داده شود، مگر عامل notifier مستقلی داشته باشد. وعدهٔ مانیتورینگ مرکزی همیشه‌روشن برای پنل خاموش داده نشود.

### تنظیمات و بکاپ

پوسته light/dark/system، زبان، timezone، refresh interval، retention، RBAC، نشست‌ها، agentها و revoked tokens، host keys، audit، نگهداری secretها و About/version/update. بکاپ به‌صورت explicit شامل یا بدون secret؛ encrypted برای secretها با سیاست کلید. پیش‌نمایش restore و mapping ایران قدیم/جدید، تشخیص تداخل، dry-run، اثر بر outside endpoint و برگشت. Export داده با backup قابل بازیابی یکی نیست.

## سناریوی افزودن تانل

1. انتخاب ایران و provider آماده. capability و نسخه بررسی شود.
2. انتخاب/افزودن خارج، IP واقعی، mappingها، alias؛ پیشنهاد نام و /30 بدون تداخل.
3. plan سمت ایران و خارج؛ رزرو lease کوتاه‌مدت برای name/network/port. routeهای هر دو میزبان و GRE endpoint pair بررسی شوند.
4. دستور enrollment خاص خارج با توکن یک‌بارمصرف کوتاه‌عمر و scope محدود. توکن در URL access log یا shell history طولانی‌مدت قرار نگیرد؛ prompt امن/ورودی stdin جایگزین داشته باشد. متن قابل کپی باید با download artifact نسخه‌دار و checksum تطبیق داشته باشد.
5. waiting/external connected/authenticated/preflight/configuring/probing/result با timeout و resume. برای میزبان مدیریت‌شده از dispatcher موجود استفاده شود.
6. H2 و H3 به تفکیک، UDP unknown مگر probe اختصاصی. پایان wizard به تست واقعی کلاینت و نتیجهٔ ذخیره‌شده منتهی شود؛ تست موفق TCP نباید user_verified را true کند.

## سناریوی انتقال پورت

ورودی: mapping ID، tunnel مقصد موجود/جدید، destination port، plan revision.

نمایش before/after و اثر بر نشست‌ها. ابتدا مقصد جدید از مسیر تانل probe شود؛ اگر fail بود مسیر فعلی دست نخورد. سپس lock میزبان + revalidate revision/reservation + backup + حذف rule قبلی و افزودن جدید به‌شکل محدود + observe readback + اتصال TCP جدید. در fail، previous desired config و منابع owned به‌صورت best-effort برگردند. rollback failure بحرانی و نیازمند اقدام دستی است. وضعیت‌های rolled_back و rollback_failed مجزا باشند.

GreFlow فعلی cross-instance transaction واحد ندارد؛ اجرای سادهٔ remove سپس add کافی برای ادعای atomic transfer نیست. پیش از فعال‌کردن این قابلیت، transaction journal و recovery در موتور اضافه شود. قوانین کنترلی از مسیرهای آزاد shell عبور نکنند.

Existing conntrack flows ممکن است به مقصد قبلی بمانند؛ reconnect راهنمایی شود. حذف selective conntrack فقط برای tuple همین ایران/protocol/public port و با انتخاب صریح؛ flush عمومی ممنوع. بررسی IP خروجی کاربر ممکن است با outbound chain برنامه فرق کند و به‌تنهایی مالکیت rule را ثابت نمی‌کند.

## طراحی بصری

| token | light | dark |
|---|---|---|
| background | #F5F3FA | #141723 |
| surface | #FFFFFF | #1D2131 |
| secondary surface | #F8F9FD | #24293B |
| text | #17213D | #EDF0FF |
| muted | #6F7891 | #A1AAC3 |
| border | #E6E8F1 | #343C53 |
| action | #386AF3 | #85A6FF |

رنگ وضعیت‌ها در CSS مرجع. نسبت کنتراست WCAG AA با تست واقعی تأیید شود؛ آیکون/متن همیشه همراه رنگ. خوانایی بر شیشه‌ای‌بودن اولویت دارد. اندازه‌ها: body 14px، caption حداقل 12px در نسخهٔ نهایی، heading 24–28px، spacing 4/8/12/16/24/32، radius 12 کنترل و 18 کارت، touch target حداقل 44px.

دسکتاپ sidebar راست 224px و grid سه ستون؛ تبلت دو ستون؛ موبایل یک ستون و منوی جمع‌شونده. جدول اسکرول افقی با ستون‌های هویتی ثابت یا تبدیل به کارت؛ نه بریدن متن. modal موبایل full-screen و footer sticky با احتساب keyboard/safe area. جهت آیکون‌های حرکتی با RTL هماهنگ، محتوای فنی همیشه LTR. کپی آدرس بدون bidi character مخفی.

فونت Vazirmatn محلی با مجوز OFL و بدون وابستگی ضروری به CDN. آیکون inline خطی. motion 150–220ms، reduced-motion، focus visible، labels و ارور مرتبط به input، aria-live برای نتیجه نه هر تغییر counter، dialog focus trap و بازگشت focus، Escape و ناوبری کیبورد. toast برای خطا کافی نیست؛ خطا در context باقی بماند.

## حالت‌های لازم هر صفحه

loading/skeleton؛ empty با اقدام مرتبط؛ filtered-empty؛ success؛ validation error؛ offline؛ stale؛ partial failure؛ permission denied؛ unsupported capability؛ operation running؛ uncertain result؛ restore/rollback result. همه با نمونهٔ fixture و تست کیبورد/موبایل طراحی شوند.

## دروازهٔ انتشار

هیچ نمونهٔ نمایشی، دستور placeholder، آمار ثابت، دکمهٔ موفق ساختگی یا token ثابت در مسیر production نباشد. Demo فقط با flag صریح و بنر همیشگی. backend health واقعی، migration نسخه‌دار، signing/checksum artifacts، backup/restore، audit/redaction، rate limit، CSRF، RBAC، reconnect و rollback test لازم‌اند.

برای public release، تمام ردیف‌های `ACCEPTANCE.md` باید مدرک داشته باشند. تا آن زمان عنوان release نباید «پنل آمادهٔ مدیریت واقعی» باشد.
