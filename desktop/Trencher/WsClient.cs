using System;
using System.Collections.ObjectModel;
using System.Net.Http;
using System.Text;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using Trencher.Models;

namespace Trencher
{
    public class WsClient
    {
        private readonly string _baseUrl;
        private ClientWebSocket? _ws;
        private CancellationTokenSource? _cts;

        public ObservableCollection<Coin> Coins { get; } = new();
        public event Action<Coin>? OnNewToken;
        public event Action<Coin>? OnMigration;

        public WsClient(string serverHost = "127.0.0.1:9090")
        {
            _baseUrl = serverHost.TrimEnd('/');
        }

        public async Task ConnectAsync()
        {
            _cts = new CancellationTokenSource();
            _ws = new ClientWebSocket();
            await _ws.ConnectAsync(new Uri($"ws://{_baseUrl}/ws"), _cts.Token);
            _ = ReceiveLoop(_cts.Token);
        }

        private async Task ReceiveLoop(CancellationToken token)
        {
            var buf = new byte[32 * 1024];
            while (_ws != null && _ws.State == WebSocketState.Open && !token.IsCancellationRequested)
            {
                using var ms = new MemoryStream();
                WebSocketReceiveResult res;
                do
                {
                    res = await _ws.ReceiveAsync(buf, token);
                    ms.Write(buf, 0, res.Count);
                } while (!res.EndOfMessage && !token.IsCancellationRequested);

                if (res.MessageType == WebSocketMessageType.Close) break;

                ms.Position = 0;
                using var doc = await JsonDocument.ParseAsync(ms, cancellationToken: token);
                var root = doc.RootElement;
                if (!root.TryGetProperty("type", out var type)) continue;

                var coin = JsonSerializer.Deserialize<Coin>(ms);
                if (coin == null) continue;

                App.Current.Dispatcher.Invoke(() =>
                {
                    Coins.Insert(0, coin);
                    if (Coins.Count > 500) Coins.RemoveAt(Coins.Count - 1);
                });

                if (type.GetString() == "migration")
                    OnMigration?.Invoke(coin);
                else
                    OnNewToken?.Invoke(coin);
            }
        }

        // Бан дева (scam) — сервер кладёт в dev_lists.
        public async Task BanDev(string wallet) =>
            await PostAsync($"http://{_baseUrl}/dev/ban?wallet={Uri.EscapeDataString(wallet)}");

        // Труст дева (good) — авто-открытие.
        public async Task TrustDev(string wallet) =>
            await PostAsync($"http://{_baseUrl}/dev/trust?wallet={Uri.EscapeDataString(wallet)}");

        private async Task PostAsync(string url)
        {
            try
            {
                using var http = new HttpClient();
                await http.PostAsync(url, null);
            }
            catch (Exception ex) { Console.WriteLine("dev op failed: " + ex.Message); }
        }
    }
}
