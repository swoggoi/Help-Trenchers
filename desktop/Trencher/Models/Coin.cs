using System;
using System.ComponentModel;
using System.Text.Json.Serialization;

namespace Trencher.Models
{
    public class Coin : INotifyPropertyChanged
    {
        private bool _autoOpen;

        [JsonPropertyName("mint")]
        public string Mint { get; set; } = "";

        [JsonPropertyName("name")]
        public string Name { get; set; } = "";

        [JsonPropertyName("symbol")]
        public string Symbol { get; set; } = "";

        [JsonPropertyName("creator")]
        public string Creator { get; set; } = "";

        [JsonPropertyName("devBuySol")]
        public string DevBuySol { get; set; } = "";

        [JsonPropertyName("devKind")]
        public string DevKind { get; set; } = ""; // "" | good | scam

        [JsonPropertyName("source")]
        public string Source { get; set; } = "";

        [JsonPropertyName("ts")]
        public long Ts { get; set; }

        public bool AutoOpen
        {
            get => _autoOpen;
            set { _autoOpen = value; OnPropertyChanged(nameof(AutoOpen)); }
        }

        // Формат ссылки на монету в axiom.trade (mint в URL).
        // Если axiom меняет формат — поправь здесь одну строку.
        public string AxiomUrl => $"https://axiom.trade/t/{Mint}";

        public string PumpUrl => $"https://pump.fun/{Mint}";

        public event PropertyChangedEventHandler? PropertyChanged;
        private void OnPropertyChanged(string n) =>
            PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(n));
    }
}
