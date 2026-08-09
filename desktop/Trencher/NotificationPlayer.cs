using System;
using System.IO;
using System.Media;
using System.Reflection;

namespace Trencher
{
    // Воспроизводит кастомный wav из папки приложения (notify.wav).
    // Если файла нет — системный бип.
    public static class NotificationPlayer
    {
        private static SoundPlayer? _player;

        public static void Init()
        {
            try
            {
                var dir = Path.GetDirectoryName(Assembly.GetExecutingAssembly().Location) ?? ".";
                var path = Path.Combine(dir, "notify.wav");
                if (File.Exists(path))
                    _player = new SoundPlayer(path);
            }
            catch { _player = null; }
        }

        public static void Play()
        {
            try
            {
                if (_player != null)
                    _player.Play();
                else
                    Console.Beep(880, 200);
            }
            catch { /* ignore */ }
        }
    }
}
