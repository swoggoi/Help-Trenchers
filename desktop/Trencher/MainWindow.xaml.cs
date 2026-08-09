using System;
using System.Diagnostics;
using System.Windows;
using System.Windows.Controls;
using Trencher.Models;

namespace Trencher
{
    public partial class MainWindow : Window
    {
        private WsClient? _ws;
        private readonly LicenseChecker _lic = new();

        public MainWindow()
        {
            InitializeComponent();
            NotificationPlayer.Init();
        }

        private async void VerifyBtn_Click(object sender, RoutedEventArgs e)
        {
            var key = KeyBox.Text.Trim();
            LicenseStatus.Text = "проверка...";
            var ok = await _lic.VerifyAsync(key);
            LicenseStatus.Text = ok ? "активна" : "невалидна";
        }

        private async void ConnectBtn_Click(object sender, RoutedEventArgs e)
        {
            try
            {
                _ws = new WsClient(ServerBox.Text.Trim());
                _ws.OnNewToken += coin =>
                {
                    // autoOpen (good-dev) или неизвестный — уведомляем
                    Notify(coin);
                };
                _ws.OnMigration += coin => Notify(coin, isMigration: true);
                DataContext = _ws;
                await _ws.ConnectAsync();
                StatusText.Text = "online";
                ConnectBtn.IsEnabled = false;
            }
            catch (Exception ex)
            {
                StatusText.Text = "ошибка: " + ex.Message;
            }
        }

        // Уведомление: открыть вкладку на axiom + кастомный звук.
        private void Notify(Coin coin, bool isMigration = false)
        {
            var label = isMigration ? "MIGRATION" : "NEW";
            StatusText.Text = $"{label}: {coin.Symbol} ({coin.Creator}) dev={coin.DevKind}";

            if (OpenBrowserChk.IsChecked == true && !string.IsNullOrEmpty(coin.Mint))
            {
                try
                {
                    // открыть в дефолтном браузере как новую вкладку
                    Process.Start(new ProcessStartInfo
                    {
                        FileName = coin.AxiomUrl,
                        UseShellExecute = true
                    });
                }
                catch (Exception ex)
                {
                    StatusText.Text = "open err: " + ex.Message;
                }
            }

            if (SoundChk.IsChecked == true)
                NotificationPlayer.Play();
        }

        private async void BanBtn_Click(object sender, RoutedEventArgs e)
        {
            if (Grid.SelectedItem is Coin c && _ws != null)
                await _ws.BanDev(c.Creator);
        }

        private async void TrustBtn_Click(object sender, RoutedEventArgs e)
        {
            if (Grid.SelectedItem is Coin c && _ws != null)
                await _ws.TrustDev(c.Creator);
        }
    }
}
