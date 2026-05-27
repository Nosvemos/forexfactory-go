from datetime import datetime
from forexfactory import ForexFactoryClient

def main():
    print("=========================================================")
    print("  FOREX FACTORY GO SDK - PYTHON DIRECT PANDAS INTEGRATION")
    print("=========================================================")
    
    try:
        # Initialize client (will auto-detect compiled libforexfactory.dll / .so)
        # We'll filter only High and Medium impact events to make it compact
        client = ForexFactoryClient(
            rate_limit=2,
            concurrency=4,
            timezone="UTC",
            impacts=["High", "Medium"]
        )
        
        start = datetime(2026, 5, 1)
        end = datetime(2026, 5, 10)
        
        print(f"Fetching range {start.strftime('%Y-%m-%d')} to {end.strftime('%Y-%m-%d')} concurrently...")
        
        # Scrape concurrently and return directly as a Pandas DataFrame!
        df = client.fetch_range(start, end, as_dataframe=True)
        
        print("\nScrape Completed Successfully!")
        print(f"Total Events Found: {len(df)}")
        
        if not df.empty:
            print("\nPandas DataFrame Head:")
            print(df[["date", "country", "impact", "title", "forecast", "actual"]].head(10))
            
            # Save to a local CSV file using Pandas
            df.to_csv("calendar_scraped_via_python.csv", index=False)
            print("\nSaved output to 'calendar_scraped_via_python.csv' using Pandas.")
        else:
            print("No events found in this date range.")
            
        client.close()
        
    except FileNotFoundError:
        print("\n[Error] Shared library DLL/SO not found.")
        print("Please compile the library first:")
        print("  Windows: run 'build_bindings.ps1' or 'make build-dll'")
        print("  Linux/Mac: run 'make build-so'")
    except Exception as e:
        print(f"\nAn error occurred: {e}")

if __name__ == "__main__":
    main()
