using System;
using System.Net.Http;
using System.Text.Json;
using System.Threading.Tasks;

namespace Trencher
{
    // Проверка лицензионного ключа через существующий бот-API (/api/verify).
    public class LicenseChecker
    {
        private readonly HttpClient _http = new();
        public string ApiBaseUrl { get; set; } = "https://help-trenchers.fly.dev";

        public async Task<bool> VerifyAsync(string key)
        {
            if (string.IsNullOrWhiteSpace(key)) return false;
            try
            {
                var url = $"{ApiBaseUrl.TrimEnd('/')}/api/verify?key={Uri.EscapeDataString(key)}";
                var resp = await _http.GetStringAsync(url);
                using var doc = JsonDocument.Parse(resp);
                if (doc.RootElement.TryGetProperty("valid", out var v))
                    return v.GetBoolean();
            }
            catch (Exception ex)
            {
                Console.WriteLine("License check failed: " + ex.Message);
            }
            return false;
        }
    }
}
