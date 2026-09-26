// Посредник до Telegram Bot API (specs/018-telegram-bot.md, требование 3б).
// С нашего VPS api.telegram.org не открывается, а Cloudflare открывается.
// Worker пересылает запрос как есть и отдаёт ответ Telegram. Токен в пути
// запроса, как и у самого Bot API, — Worker его не хранит и не пишет.
//
// Как завести: dash.cloudflare.com → Workers & Pages → Create → Worker,
// заменить код этим файлом, Deploy. Адрес вида
// https://<имя>.<аккаунт>.workers.dev положить в переменную репозитория
// TELEGRAM_API_URL и запустить деплой.
export default {
  async fetch(request) {
    const url = new URL(request.url);
    if (!url.pathname.startsWith("/bot")) {
      return new Response("not found", { status: 404 });
    }
    const target = "https://api.telegram.org" + url.pathname + url.search;
    return fetch(new Request(target, request));
  },
};
