from datetime import datetime
from tradingview_calendar import TradingViewCalendarClient

def main():
    print("================================================================")
    print("  TRADINGVIEW CALENDAR GO SDK - PYTHON DIRECT PANDAS INTEGRATION")
    print("================================================================")
    
    try:
        # Initialize client (will auto-detect compiled libtvcalendar.dll / .so)
        client = TradingViewCalendarClient(
            rate_limit=10,
            concurrency=5,
            timezone="UTC",
            impacts=["High", "Medium"]
        )
        
        start = datetime(2024, 1, 1)
        end = datetime(2024, 1, 15)
        
        print(f"Fetching range {start.strftime('%Y-%m-%d')} to {end.strftime('%Y-%m-%d')} concurrently...")
        
        # Download concurrently and return directly as a Pandas DataFrame!
        df = client.fetch_range(start, end, as_dataframe=True)
        
        print("\nDownload Completed Successfully!")
        print(f"Total Events Found: {len(df)}")
        
        if not df.empty:
            print("\nPandas DataFrame Head:")
            print(df[["date", "country", "currency", "impact", "title", "forecast", "actual"]].head(10))
            
            df.to_csv("calendar_via_python.csv", index=False)
            print("\nSaved output to 'calendar_via_python.csv' using Pandas.")
        else:
            print("No events found in this date range.")
            
        client.close()
        
    except FileNotFoundError:
        print("\n[Error] Shared library DLL/SO not found.")
        print("Please compile the library first:")
        print("  Windows: make build-dll")
        print("  Linux/Mac: make build-so")
    except Exception as e:
        print(f"\nAn error occurred: {e}")

if __name__ == "__main__":
    main()
