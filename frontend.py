# frontend.py

# -*- coding: utf-8 -*- 

import sys
import os
import threading
import subprocess
import platform
import ping3
import requests
import socket
import time  # To track real time

from PySide6.QtWidgets import (
    QApplication, QMainWindow, QPushButton, QLabel, QVBoxLayout, QWidget,
    QGraphicsDropShadowEffect, QSystemTrayIcon, QMenu, QDialog
)
from PySide6.QtGui import (
    QPixmap, QFont, QIcon, QCursor, QPainter, QColor, QRadialGradient
)
from PySide6.QtCore import (
    Qt, QTimer, QSize, QPropertyAnimation, QEasingCurve, QPoint
)

# ------------------ Utility ------------------
def get_proxy_latency():
    try:
        proxies = {
            "http": "http://127.0.0.1:4525",
            "https": "http://127.0.0.1:4525",
        }
        r = requests.get("https://www.youtube.com", proxies=proxies, timeout=3)
        return r.elapsed.total_seconds() * 1000
    except Exception:
        return None

# ------------------ Resource Path ------------------
def resource_path(relative_path):
    try:
        base_path = sys._MEIPASS
    except Exception:
        base_path = os.path.abspath(".")
    return os.path.join(base_path, relative_path)

# ------------------ Single Instance Lock ------------------
def already_running():
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    try:
        sock.bind(('127.0.0.1', 54321))
        return False
    except socket.error:
        return True

# ------------------ Cross-platform Chrome Path ------------------
def get_chrome_path():
    system = platform.system()
    if system == "Windows":
        return r"C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe"
    elif system == "Linux":
        return "/usr/bin/google-chrome"
    return None

# ------------------ Launch Chrome ------------------
def launch_chrome_with_proxy(proxy_host, port, profile_dir):
    chrome_path = get_chrome_path()
    if not chrome_path or not os.path.exists(chrome_path):
        raise FileNotFoundError(f"Chrome not found at {chrome_path}")

    cmd = [
        chrome_path,
        f"--user-data-dir={profile_dir}",
        f"--proxy-server={proxy_host}:{port}",
        "--proxy-bypass-list=<-loopback>",
        "--no-first-run",
        "--disable-gpu",
        "--disable-software-rasterizer",
        "https://www.youtube.com/"
    ]
    return subprocess.Popen(cmd)

# ------------------ Assets ------------------
ASSETS = {
    'icon_on': resource_path("Assets/power_icon-on.svg"),
    'icon_off': resource_path("Assets/power_icon-off.svg"),
    'icon_connecting': resource_path("Assets/power-icon-connecting.svg"),
    'logo': resource_path("Assets/logo.png"),
    'icon_about': resource_path("Assets/about.svg"),
    'icon_minimize': resource_path("Assets/minimize.svg"),
    'icon_close': resource_path("Assets/close.svg"),
    'bg': resource_path("Assets/app-bg.png"),
    'about_bg': resource_path("Assets/about-bg.png"),
    'about_logo': resource_path("Assets/about_logo.png")
}

# ------------------ Custom Widgets ------------------
class CircleButton(QPushButton):
    def __init__(self):
        super().__init__()
        self.setFixedSize(220, 220)
        self.setAccessibleName("Power Button")
        self.setCursor(QCursor(Qt.PointingHandCursor))
        self.setIconSize(QSize(120, 120))

        self.shadow = QGraphicsDropShadowEffect(self)
        self.shadow.setBlurRadius(48)
        self.shadow.setOffset(0, 10)
        self.setGraphicsEffect(self.shadow)

        self.glow = QPropertyAnimation(self.shadow, b"blurRadius")
        self.glow.setStartValue(24)
        self.glow.setEndValue(48)
        self.glow.setDuration(350)
        self.glow.setEasingCurve(QEasingCurve.InOutQuad)

        self.pulse = QPropertyAnimation(self, b"size")
        self.pulse.setStartValue(self.size())
        self.pulse.setEndValue(QSize(self.width() * 1.05, self.height() * 1.05))
        self.pulse.setDuration(600)
        self.pulse.setLoopCount(-1)
        self.pulse.setEasingCurve(QEasingCurve.InOutQuad)

        self._mode = "inactive"
        self.set_icon_color("inactive")

    def set_icon_color(self, mode):
        self._mode = mode
        if mode in ("active", "connecting"):
            self.glow.start()
            self.pulse.start()
        else:
            self.glow.stop()
            self.pulse.stop()
        self.update()

    def paintEvent(self, ev):
        p = QPainter(self)
        p.setRenderHint(QPainter.Antialiasing)
        grad = QRadialGradient(self.width()/2, self.height()/2, self.width()/2)
        grad.setColorAt(0, {
            "active": QColor(0, 255, 170, 200),
            "connecting": QColor(255, 170, 0, 220),
            "inactive": QColor(0, 255, 170, 50)
        }[self._mode])
        grad.setColorAt(1, QColor(0, 255, 170, 0))
        p.setBrush(grad)
        p.setPen(Qt.NoPen)
        p.drawEllipse(0, 0, self.width(), self.height())

        p.setBrush(QColor(255, 255, 255))
        ir = self.width()*0.75
        ix = (self.width()-ir)/2
        iy = (self.height()-ir)/2
        p.drawEllipse(ix, iy, ir, ir)

        icon = QIcon({
            "active": ASSETS['icon_on'],
            "connecting": ASSETS['icon_connecting'],
        }.get(self._mode, ASSETS['icon_off'])).pixmap(120, 120)
        p.drawPixmap(ix + (ir - 120) / 2, iy + (ir - 120) / 2, icon)

# ------------------ About Dialog ------------------
class AboutDialog(QDialog):
    def __init__(self, parent=None):
        super().__init__(parent)
        self.setWindowFlags(Qt.FramelessWindowHint | Qt.Dialog)
        self.setAttribute(Qt.WA_TranslucentBackground)
        self.setModal(True)
        self.setFixedSize(340, 270)

        self.bg_frame = QWidget(self)
        self.bg_frame.setGeometry(0, 0, 340, 270)
        self.bg_frame.setStyleSheet(""" 
            QWidget { 
                background: white; 
                border-radius: 18px; 
                border: 2px solid #ff0000; 
            } 
        """)

        layout = QVBoxLayout(self.bg_frame)
        layout.setContentsMargins(18, 18, 18, 18)
        layout.setAlignment(Qt.AlignTop | Qt.AlignHCenter)

        logo = QLabel()
        if os.path.exists(ASSETS['about_logo']):
            logo.setPixmap(QPixmap(ASSETS['about_logo']).scaled(48, 48, Qt.KeepAspectRatio, Qt.SmoothTransformation))
        else:
            logo.setText("YOUNBLOCK")
            logo.setFont(QFont("Segoe UI", 18, QFont.Bold))
            logo.setStyleSheet("color:#ff0000;")
        logo.setAlignment(Qt.AlignCenter)
        layout.addWidget(logo)

        desc = QLabel(""" 
            <h3 style='color:#ff0000;'>What is YOUNBLOCK?</h3> 
            <p style='color:#ff0000;'>YOUNBLOCK (v1.0 by Chita) lets you access YouTube faster than most VPNs<br> 
            by blocking DNS manipulation. It does not hide your IP and only works<br> 
            with Google Chrome.</p> 
        """)
        desc.setWordWrap(True)
        desc.setAlignment(Qt.AlignCenter)
        desc.setStyleSheet("font-size:14px; color:#ff0000;")
        layout.addWidget(desc)

        btn = QPushButton("Close")
        btn.setFixedWidth(100)
        btn.setCursor(QCursor(Qt.PointingHandCursor))
        btn.setStyleSheet(""" 
            QPushButton { 
                background: #fff; color: #ff0000; 
                border-radius: 10px; border:1px solid #ff0000; 
                padding:8px 20px; font-size:13px; 
            } 
            QPushButton:hover { 
                background: #ff0000; color: #fff; 
            } 
        """)
        btn.clicked.connect(self.accept)
        layout.addWidget(btn, alignment=Qt.AlignCenter)

# ------------------ Main App ------------------
from backend import run_backend

class ProxyApp(QMainWindow):
    def __init__(self):
        super().__init__()
        self.setWindowTitle("YOUNBLOCK")
        self.setFixedSize(480, 640)
        self.chrome_process = None
        self.session_seconds = 0
        self.proxy_status = False
        self.data_consumed = 0  # Track data consumption in bytes
        self.start_time = None  # For accurate timer tracking

        self.init_ui()
        self.init_tray()
        self.init_timers()

    def init_ui(self):
        central = QWidget()
        self.setCentralWidget(central)
        layout = QVBoxLayout(central)
        layout.setContentsMargins(20, 20, 20, 20)
        layout.setSpacing(16)

        # Dark theme styling
        self.setStyleSheet("""
            QMainWindow {
                background-color: #2E2E2E;
                color: #D1D1D1;
            }
            QLabel {
                color: #D1D1D1;
            }
            QPushButton {
                background-color: #444;
                color: #D1D1D1;
                border-radius: 10px;
                padding: 10px 20px;
            }
            QPushButton:hover {
                background-color: #555;
            }
        """)

        self.logo = QLabel()
        self.logo.setAlignment(Qt.AlignCenter)
        if os.path.exists(ASSETS['logo']):
            self.logo.setPixmap(QPixmap(ASSETS['logo']))
        else:
            self.logo.setText("YOUNBLOCK")
            self.logo.setFont(QFont("Segoe UI", 20))
        layout.addWidget(self.logo)

        self.power_button = CircleButton()
        self.power_button.clicked.connect(self.toggle_proxy)
        layout.addWidget(self.power_button, alignment=Qt.AlignCenter)

        self.status_label = QLabel("Disconnected")
        self.status_label.setAlignment(Qt.AlignCenter)
        self.status_label.setFont(QFont("Segoe UI", 18, QFont.Bold))  # Increased font size and bold
        self.status_label.setStyleSheet("color: #ff3333;")  # Initially red for Disconnected
        layout.addWidget(self.status_label)

        self.timer_label = QLabel("00:00:00")
        self.timer_label.setAlignment(Qt.AlignCenter)
        self.timer_label.setFont(QFont("Segoe UI", 11))
        layout.addWidget(self.timer_label)

        self.ping_label = QLabel("Ping: -- ms")
        self.ping_label.setAlignment(Qt.AlignCenter)
        self.ping_label.setFont(QFont("Segoe UI", 10))
        layout.addWidget(self.ping_label)

        # New Bandwidth Label
        self.bandwidth_label = QLabel("Bandwidth: 0 MB")
        self.bandwidth_label.setAlignment(Qt.AlignCenter)
        self.bandwidth_label.setFont(QFont("Segoe UI", 10))
        layout.addWidget(self.bandwidth_label)

        self.footer_label = QLabel('<a href="#" style="color:#ffb6c1;">Compatible Only With Google Chrome – V1.0 by Chita.</a>')
        self.footer_label.setOpenExternalLinks(False)
        self.footer_label.linkActivated.connect(self.show_about_dialog)
        self.footer_label.setAlignment(Qt.AlignCenter)
        layout.addWidget(self.footer_label)

    def init_tray(self):
        self.tray_icon = QSystemTrayIcon(QIcon(ASSETS['icon_off']), self)
        self.tray_menu = QMenu(self)
        self.tray_menu.addAction("Connect", self.connect_proxy)  # Added Connect option
        self.tray_menu.addAction("Disconnect", self.disconnect_proxy)  # Added Disconnect option
        self.tray_menu.addAction("Exit", QApplication.instance().quit)
        self.tray_icon.setContextMenu(self.tray_menu)
        self.tray_icon.show()

    def connect_proxy(self):
        self.toggle_proxy()

    def disconnect_proxy(self):
        self.toggle_proxy()

    def update_tray_icon(self, mode):
        icon = {
            "active": ASSETS['icon_on'],
            "connecting": ASSETS['icon_connecting'],
            "inactive": ASSETS['icon_off']
        }.get(mode, ASSETS['icon_off'])
        self.tray_icon.setIcon(QIcon(icon))

    def toggle_proxy(self):
        if not self.proxy_status:
            self.status_label.setText("Connecting...")
            self.status_label.setStyleSheet("color:#ffaa00;")
            self.power_button.set_icon_color("connecting")
            self.update_tray_icon("connecting")
            QTimer.singleShot(1200, self.launch_chrome_after_green)
            self.proxy_status = True

            # Start timer when connected
            self.start_time = time.time()  # Set the start time
            self.timer.start(1000)  # Start the timer every second
        else:
            self.status_label.setText("Disconnecting...")
            self.status_label.setStyleSheet("color:#ff3333;")
            if self.chrome_process:
                self.chrome_process.terminate()
                self.chrome_process = None
            self.power_button.set_icon_color("inactive")
            self.update_tray_icon("inactive")
            self.status_label.setText("Disconnected")
            self.status_label.setStyleSheet("color:#D9534F;")  # Color remains red for Disconnected
            self.timer.stop()
            self.session_seconds = 0
            self.timer_label.setText("00:00:00")
            self.signal_timer.stop()
            self.ping_label.setText("Ping: -- ms")
            self.bandwidth_label.setText("Bandwidth: 0 MB")  # Reset bandwidth
            self.proxy_status = False

    def launch_chrome_after_green(self):
        try:
            self.chrome_process = launch_chrome_with_proxy("127.0.0.1", 4525, os.path.abspath("proxy-profile"))
            self.status_label.setText("Connected")
            self.status_label.setStyleSheet("color:#28A745;")
            self.power_button.set_icon_color("active")
            self.update_tray_icon("active")
            self.timer.start(1000)
            self.signal_timer.start(5000)
        except Exception as e:
            self.status_label.setText(f"Error: {e}")
            self.status_label.setStyleSheet("color:#ff3333;")
            self.update_tray_icon("inactive")

    def init_timers(self):
        self.timer = QTimer(self)
        self.timer.timeout.connect(self.update_timer)
        self.signal_timer = QTimer(self)
        self.signal_timer.timeout.connect(self.update_ping)

        # New timer for bandwidth update
        self.bandwidth_timer = QTimer(self)
        self.bandwidth_timer.timeout.connect(self.update_bandwidth)
        self.bandwidth_timer.start(5000)  # Update bandwidth every 5 seconds

    def update_timer(self):
        self.session_seconds += 1
        hrs, rem = divmod(self.session_seconds, 3600)
        mins, secs = divmod(rem, 60)
        self.timer_label.setText(f"{hrs:02d}:{mins:02d}:{secs:02d}")

    def update_ping(self):
        try:
            icmp = ping3.ping("dns.google")
            proxy_latency = get_proxy_latency()
            results = []
            timeout = False

            if icmp is not None:
                results.append(f"Google DNS: {int(icmp * 1000)} ms")
            else:
                results.append("Google DNS: timeout")
                timeout = True

            if proxy_latency is not None:
                results.append(f"Proxy: {int(proxy_latency)} ms")
            else:
                results.append("Proxy: timeout")
                timeout = True

            # Set color to red if timeout occurred
            if timeout:
                self.ping_label.setStyleSheet("color: red;")
            else:
                self.ping_label.setStyleSheet("color: black;")

            self.ping_label.setText(" | ".join(results))
        except Exception:
            self.ping_label.setText("Ping: error")
            self.ping_label.setStyleSheet("color: red;")

    def update_bandwidth(self):
        # Simulate data usage increment every 5 seconds (in bytes)
        data_increment = self.simulate_data_usage()  # Simulate data usage
        self.data_consumed += data_increment  # Add to total data consumed

        # Convert the bytes to a human-readable unit (MB/GB)
        bandwidth_display = self.convert_bytes_to_human_readable(self.data_consumed)
        self.bandwidth_label.setText(f"Bandwidth: {bandwidth_display}")

    def simulate_data_usage(self):
        # Simulate data usage (e.g., 100 KB every 5 seconds)
        import random
        return random.randint(100000, 1000000)  # Random data between 100KB and 1MB per interval

    def convert_bytes_to_human_readable(self, bytes):
        # Convert bytes to MB/GB
        if bytes >= 1073741824:  # GB
            return f"{bytes / 1073741824:.2f} GB"
        elif bytes >= 1048576:  # MB
            return f"{bytes / 1048576:.2f} MB"
        elif bytes >= 1024:  # KB
            return f"{bytes / 1024:.2f} KB"
        else:  # Bytes
            return f"{bytes} Bytes"

    def show_about_dialog(self):
        dlg = AboutDialog(self)
        geo = self.geometry()
        center = QPoint((geo.width() - dlg.width()) // 2, (geo.height() - dlg.height()) // 2)
        dlg.move(self.mapToGlobal(center))
        dlg.exec()

# ------------------ Main Entry ------------------
if __name__ == "__main__":
    if already_running():
        sys.exit("Another instance is already running.")

    threading.Thread(target=run_backend, daemon=True).start()
    app = QApplication(sys.argv)
    window = ProxyApp()
    window.show()
    sys.exit(app.exec())
