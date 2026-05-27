import ctypes
import json
import os
import platform
from datetime import datetime

class ForexFactoryClient:
    """
    ForexFactoryClient wraps the high-performance Go forexfactory-go library
    using ctypes C-bindings, providing sub-second historical scrapes and live trackers
    directly to Python and Pandas workflows.
    """
    
    def __init__(self, dll_path=None, user_agent=None, proxy_url=None, rate_limit=1, concurrency=3, timezone=None, impacts=None):
        """
        Initializes the client.
        
        :param dll_path: Path to libforexfactory.dll / libforexfactory.so. 
                         If None, searches the current directory and parent directory.
        :param user_agent: Custom User-Agent string.
        :param proxy_url: Custom Proxy URL (HTTP/SOCKS5).
        :param rate_limit: Max requests per second.
        :param concurrency: Number of concurrent downloader workers.
        :param timezone: Target timezone (e.g. 'UTC', 'America/New_York').
        :param impacts: List of impacts to filter (e.g., ['High', 'Medium']).
        """
        if dll_path is None:
            # Autodetect shared library name based on OS
            system = platform.system().lower()
            if system == 'windows':
                lib_name = 'libforexfactory.dll'
            elif system == 'darwin':
                lib_name = 'libforexfactory.dylib'
            else:
                lib_name = 'libforexfactory.so'
            
            # Search paths
            search_paths = [
                os.path.join(os.getcwd(), lib_name),
                os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', lib_name),
                os.path.join(os.path.dirname(os.path.abspath(__file__)), lib_name),
            ]
            
            for path in search_paths:
                if os.path.exists(path):
                    dll_path = path
                    break
                    
            if dll_path is None:
                raise FileNotFoundError(
                    f"Could not autodetect {lib_name}. Please compile it using `make build-so` / `make build-dll` "
                    f"and provide the direct `dll_path` argument."
                )

        # Load shared library
        self.lib = ctypes.CDLL(dll_path)
        
        # Configure ctypes function signatures
        self.lib.InitClient.argtypes = [ctypes.c_char_p]
        self.lib.InitClient.restype = ctypes.c_longlong
        
        self.lib.FreeClient.argtypes = [ctypes.c_longlong]
        self.lib.FreeClient.restype = None
        
        self.lib.FetchWeekJSON.argtypes = [ctypes.c_longlong, ctypes.c_longlong]
        self.lib.FetchWeekJSON.restype = ctypes.c_void_p
        
        self.lib.FetchRangeJSON.argtypes = [ctypes.c_longlong, ctypes.c_longlong, ctypes.c_longlong]
        self.lib.FetchRangeJSON.restype = ctypes.c_void_p
        
        self.lib.FreeString.argtypes = [ctypes.c_void_p]
        self.lib.FreeString.restype = None

        # Build configurations payload
        config = {
            "user_agent": user_agent or "",
            "proxy_url": proxy_url or "",
            "rate_limit": rate_limit,
            "concurrency": concurrency,
            "timezone": timezone or "",
            "impacts": impacts or []
        }
        
        config_bytes = json.dumps(config).encode('utf-8')
        
        # Instantiate Go Client and store opaque handle
        self.handle = self.lib.InitClient(config_bytes)
        if self.handle <= 0:
            raise RuntimeError("Failed to initialize Go ForexFactory Client via CGO bindings.")

    def fetch_week(self, date: datetime) -> list:
        """
        Fetches calendar events for the week containing the specified date.
        
        :param date: datetime object.
        :return: List of events.
        """
        ts = int(date.timestamp())
        res_ptr = self.lib.FetchWeekJSON(self.handle, ts)
        if not res_ptr:
            return []
            
        try:
            res_str = ctypes.string_at(res_ptr).decode('utf-8')
            data = json.loads(res_str)
            if isinstance(data, dict) and "error" in data:
                raise RuntimeError(data["error"])
            return data
        finally:
            self.lib.FreeString(res_ptr)

    def fetch_range(self, start_date: datetime, end_date: datetime, as_dataframe: bool = True):
        """
        Fetches calendar events concurrently spanning the range between start_date and end_date.
        
        :param start_date: datetime object.
        :param end_date: datetime object.
        :param as_dataframe: If True, returns a structured Pandas DataFrame. If False, returns raw JSON list.
        :return: List of dicts or Pandas DataFrame.
        """
        start_ts = int(start_date.timestamp())
        end_ts = int(end_date.timestamp())
        
        res_ptr = self.lib.FetchRangeJSON(self.handle, start_ts, end_ts)
        if not res_ptr:
            return pd.DataFrame() if as_dataframe else []
            
        try:
            res_str = ctypes.string_at(res_ptr).decode('utf-8')
            data = json.loads(res_str)
            if isinstance(data, dict) and "error" in data:
                raise RuntimeError(data["error"])
            
            if as_dataframe:
                try:
                    import pandas as pd
                    df = pd.DataFrame(data)
                    if not df.empty and "date" in df.columns:
                        df["date"] = pd.to_datetime(df["date"])
                    return df
                except ImportError:
                    # Fallback to dictionary list if pandas is not installed
                    return data
            return data
        finally:
            self.lib.FreeString(res_ptr)

    def close(self):
        """
        Closes the client and safely frees all browser resources.
        """
        if hasattr(self, 'handle') and self.handle > 0:
            self.lib.FreeClient(self.handle)
            self.handle = 0

    def __del__(self):
        self.close()
